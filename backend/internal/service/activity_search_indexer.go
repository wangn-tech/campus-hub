package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/wangn-tech/campus-hub/internal/model"
	espkg "github.com/wangn-tech/campus-hub/internal/platform/elasticsearch"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const SearchIndexerConsumerGroup = "campushub.search-indexer"

func ActivityPhysicalIndex(prefix string) string { return prefix + "_v1" }

type activitySearchEvent struct {
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	Payload     struct {
		ActivityID string `json:"activity_id"`
	} `json:"payload"`
}

type activitySearchDocument struct {
	ID                       string    `json:"id"`
	Title                    string    `json:"title"`
	TitleKeyword             string    `json:"title_keyword"`
	Description              string    `json:"description"`
	CategoryID               string    `json:"category_id"`
	CategoryName             string    `json:"category_name"`
	TagIDs                   []string  `json:"tag_ids"`
	TagNames                 []string  `json:"tag_names"`
	OrganizerID              string    `json:"organizer_id"`
	OrganizerName            string    `json:"organizer_name"`
	Location                 string    `json:"location"`
	AddressDetail            string    `json:"address_detail"`
	GeoPoint                 *geoPoint `json:"geo_point,omitempty"`
	Status                   uint8     `json:"status"`
	RegisterStartAt          int64     `json:"register_start_at"`
	RegisterEndAt            int64     `json:"register_end_at"`
	ActivityStartAt          int64     `json:"activity_start_at"`
	ActivityEndAt            int64     `json:"activity_end_at"`
	MaxParticipants          uint32    `json:"max_participants"`
	ApprovedParticipantCount uint32    `json:"approved_participant_count"`
	PendingParticipantCount  uint32    `json:"pending_participant_count"`
	ViewCount                uint64    `json:"view_count"`
	Version                  uint32    `json:"version"`
	Deleted                  bool      `json:"deleted"`
	CreatedAt                int64     `json:"created_at"`
	UpdatedAt                int64     `json:"updated_at"`
}

type geoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// ActivitySearchIndexer materializes the activity search read model from
// reliable activity events. It always reloads MySQL, which makes replayed and
// compact event payloads safe while external document versioning prevents an
// old record from replacing a newer representation.
type ActivitySearchIndexer struct {
	activities *repository.ActivityRepository
	categories *repository.CategoryRepository
	users      *repository.UserRepository
	client     *espkg.Client
	alias      string
	logger     *zap.Logger
}

func NewActivitySearchIndexer(activities *repository.ActivityRepository, categories *repository.CategoryRepository, users *repository.UserRepository, client *espkg.Client, alias string, logger *zap.Logger) *ActivitySearchIndexer {
	return &ActivitySearchIndexer{activities: activities, categories: categories, users: users, client: client, alias: alias, logger: logger}
}

func (i *ActivitySearchIndexer) HandleEvent(ctx context.Context, record kafka.Record) error {
	var event activitySearchEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		i.logger.Warn("skipping malformed activity search event", zap.Error(err))
		return nil
	}
	if len(event.EventType) < len("activity.") || event.EventType[:len("activity.")] != "activity." {
		return nil
	}
	id := event.Payload.ActivityID
	if id == "" {
		id = event.AggregateID
	}
	activity, err := i.activities.FindByUUID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	document, err := i.document(ctx, activity)
	if err != nil {
		return err
	}
	return i.client.Index(ctx, i.alias, activity.UUID, activity.Version, document)
}

func (i *ActivitySearchIndexer) document(ctx context.Context, activity *model.Activity) (activitySearchDocument, error) {
	document := activitySearchDocument{ID: activity.UUID, Title: activity.Title, TitleKeyword: activity.Title, Description: activity.Description, Location: activity.Location, AddressDetail: activity.AddressDetail, Status: activity.Status, RegisterStartAt: timestamp.ToMillis(activity.RegisterStartAt), RegisterEndAt: timestamp.ToMillis(activity.RegisterEndAt), ActivityStartAt: timestamp.ToMillis(activity.ActivityStartAt), ActivityEndAt: timestamp.ToMillis(activity.ActivityEndAt), MaxParticipants: activity.MaxParticipants, ApprovedParticipantCount: activity.ApprovedParticipantCount, PendingParticipantCount: activity.PendingParticipantCount, ViewCount: activity.ViewCount, Version: activity.Version, Deleted: activity.DeletedAt != nil || !isPublicActivityStatus(activity.Status), CreatedAt: timestamp.ToMillis(activity.CreatedAt), UpdatedAt: timestamp.ToMillis(activity.UpdatedAt)}
	if activity.Latitude != nil && activity.Longitude != nil {
		document.GeoPoint = &geoPoint{Lat: *activity.Latitude, Lon: *activity.Longitude}
	}
	categories, err := i.categories.FindByIDs(ctx, []uint64{activity.CategoryID})
	if err != nil {
		return document, err
	}
	if len(categories) > 0 {
		document.CategoryID, document.CategoryName = categories[0].UUID, categories[0].Name
	}
	users, err := i.users.FindByIDs(ctx, []uint64{activity.OrganizerID})
	if err != nil {
		return document, err
	}
	if len(users) > 0 {
		document.OrganizerID, document.OrganizerName = users[0].UUID, users[0].Nickname
	}
	tags, err := i.activities.TagsForActivityIDs(ctx, []uint64{activity.ID})
	if err != nil {
		return document, err
	}
	for _, tag := range tags[activity.ID] {
		document.TagIDs = append(document.TagIDs, tag.UUID)
		document.TagNames = append(document.TagNames, tag.Name)
	}
	return document, nil
}

// Rebuild writes the complete MySQL source into a new physical index. The
// caller validates success and switches the alias only after this returns nil.
type RebuildResult struct {
	Count     int
	SampleIDs []string
}

func (i *ActivitySearchIndexer) Rebuild(ctx context.Context, index string, batchSize int) (RebuildResult, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	result := RebuildResult{}
	var afterID uint64
	for {
		activities, err := i.activities.ListForSearchRebuild(ctx, afterID, batchSize)
		if err != nil {
			return result, err
		}
		if len(activities) == 0 {
			return result, nil
		}
		for pos := range activities {
			activity := &activities[pos]
			document, err := i.document(ctx, activity)
			if err != nil {
				return result, err
			}
			if err := i.client.Index(ctx, index, activity.UUID, activity.Version, document); err != nil {
				return result, err
			}
			result.Count++
			if len(result.SampleIDs) < 3 {
				result.SampleIDs = append(result.SampleIDs, activity.UUID)
			}
		}
		afterID = activities[len(activities)-1].ID
	}
}
