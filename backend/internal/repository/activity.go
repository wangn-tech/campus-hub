package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

// ErrConcurrentUpdate is returned when a guarded write did not match the
// expected row state, meaning someone else changed it first.
var ErrConcurrentUpdate = errors.New("concurrent update")

type ActivityRepository struct{ db *gorm.DB }

func NewActivityRepository(db *gorm.DB) *ActivityRepository { return &ActivityRepository{db: db} }

// ActivityFilter describes the supported list and search filters.
type ActivityFilter struct {
	Statuses    []int
	CategoryID  *uint64
	TagID       *uint64
	OrganizerID *uint64
	Keyword     string
	Location    string
	StartFrom   *time.Time
	StartTo     *time.Time
	Sort        string
	Page        int
	PageSize    int
}

func (r *ActivityRepository) Create(ctx context.Context, activity *model.Activity, tagIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(activity).Error; err != nil {
			return err
		}
		return replaceActivityTags(tx, activity.ID, tagIDs)
	})
}

func (r *ActivityRepository) FindByUUID(ctx context.Context, id string) (*model.Activity, error) {
	var activity model.Activity
	if err := r.db.WithContext(ctx).Where("uuid = ? AND deleted_at IS NULL", id).First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *ActivityRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Activity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var activities []model.Activity
	err := r.db.WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ids).Find(&activities).Error
	return activities, err
}

func (r *ActivityRepository) FindByID(ctx context.Context, id uint64) (*model.Activity, error) {
	var activity model.Activity
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *ActivityRepository) List(ctx context.Context, filter ActivityFilter) ([]model.Activity, int64, error) {
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&model.Activity{}).Where("activities.deleted_at IS NULL")
		if len(filter.Statuses) > 0 {
			query = query.Where("activities.status IN ?", filter.Statuses)
		}
		if filter.CategoryID != nil {
			query = query.Where("activities.category_id = ?", *filter.CategoryID)
		}
		if filter.OrganizerID != nil {
			query = query.Where("activities.organizer_id = ?", *filter.OrganizerID)
		}
		if filter.TagID != nil {
			query = query.Where("EXISTS (SELECT 1 FROM activity_tag_relations r WHERE r.activity_id = activities.id AND r.tag_id = ?)", *filter.TagID)
		}
		if filter.Keyword != "" {
			like := "%" + filter.Keyword + "%"
			query = query.Where("(activities.title LIKE ? OR activities.location LIKE ? OR activities.description LIKE ?)", like, like, like)
		}
		if filter.Location != "" {
			query = query.Where("activities.location LIKE ?", "%"+filter.Location+"%")
		}
		if filter.StartFrom != nil {
			query = query.Where("activities.activity_start_at >= ?", *filter.StartFrom)
		}
		if filter.StartTo != nil {
			query = query.Where("activities.activity_start_at <= ?", *filter.StartTo)
		}
		return query
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Activity
	err := base().Order(activityOrder(filter.Sort)).
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update writes the given values guarded by the activity version. When log is
// not nil the status log is written in the same transaction.
func (r *ActivityRepository) Update(ctx context.Context, activity *model.Activity, values map[string]any, tagIDs []uint64, log *model.ActivityStatusLog) error {
	values["version"] = activity.Version + 1
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Activity{}).
			Where("id = ? AND version = ?", activity.ID, activity.Version).
			Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConcurrentUpdate
		}
		if err := replaceActivityTags(tx, activity.ID, tagIDs); err != nil {
			return err
		}
		if log != nil {
			return tx.Create(log).Error
		}
		return nil
	})
}

// ActivityTransitionInput is the atomic unit of work for an activity status
// change, including the optional activity group side effects.
type ActivityTransitionInput struct {
	ActivityID uint64
	FromStatus uint8
	Values     map[string]any
	Log        *model.ActivityStatusLog
	// Publishing an activity also creates its chat group with the organizer as
	// owner, in the same transaction.
	Group      *model.ChatGroup
	GroupOwner *model.ChatGroupMember
	// Cancelling an activity dissolves its chat group.
	DissolveGroup bool
}

// Transition applies a guarded status change and records the status log in the
// same transaction. It returns ErrConcurrentUpdate when the row was not in the
// expected source status.
func (r *ActivityRepository) Transition(ctx context.Context, in ActivityTransitionInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Activity{}).
			Where("id = ? AND status = ?", in.ActivityID, in.FromStatus).
			Updates(in.Values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConcurrentUpdate
		}
		if in.Group != nil {
			if err := createGroupWithOwner(tx, in.Group, in.GroupOwner); err != nil {
				return err
			}
		}
		if in.DissolveGroup {
			err := tx.Model(&model.ChatGroup{}).Where("activity_id = ?", in.ActivityID).
				Updates(map[string]any{"status": model.ChatGroupDissolved, "updated_at": time.Now().UTC()}).Error
			if err != nil {
				return err
			}
		}
		if in.Log != nil {
			return tx.Create(in.Log).Error
		}
		return nil
	})
}

func (r *ActivityRepository) AddStatusLog(ctx context.Context, log *model.ActivityStatusLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *ActivityRepository) IncrementViewCount(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Activity{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// SyncStatuses moves published/ongoing activities to their time based status
// and returns how many rows changed.
func (r *ActivityRepository) SyncStatuses(ctx context.Context, now time.Time) (int64, error) {
	var changed int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []model.Activity
		err := tx.Select("id", "status", "activity_start_at", "activity_end_at").
			Where("deleted_at IS NULL AND status IN ?", []int{int(model.ActivityPublished), int(model.ActivityOngoing)}).
			Where("activity_start_at <= ? OR activity_end_at <= ?", now, now).
			Find(&candidates).Error
		if err != nil {
			return err
		}
		for _, activity := range candidates {
			to, ok := nextTimedStatus(model.ActivityStatus(activity.Status), activity.ActivityStartAt, activity.ActivityEndAt, now)
			if !ok {
				continue
			}
			result := tx.Model(&model.Activity{}).
				Where("id = ? AND status = ?", activity.ID, activity.Status).
				Update("status", uint8(to))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			log := &model.ActivityStatusLog{
				UUID:         uuid.NewString(),
				ActivityID:   activity.ID,
				FromStatus:   activity.Status,
				ToStatus:     uint8(to),
				OperatorType: model.ActivityOperatorSystem,
				Reason:       "time based transition",
				CreatedAt:    now,
			}
			if err := tx.Create(log).Error; err != nil {
				return err
			}
			changed++
		}
		return nil
	})
	return changed, err
}

func (r *ActivityRepository) TagsForActivityIDs(ctx context.Context, ids []uint64) (map[uint64][]model.Tag, error) {
	result := make(map[uint64][]model.Tag, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []activityTagRow
	err := r.db.WithContext(ctx).Table("activity_tag_relations").
		Select("activity_tag_relations.activity_id AS activity_id, tags.id AS id, tags.uuid AS uuid, tags.name AS name, tags.slug AS slug, tags.color AS color, tags.icon AS icon, tags.status AS status").
		Joins("JOIN tags ON tags.id = activity_tag_relations.tag_id").
		Where("activity_tag_relations.activity_id IN ?", ids).
		Order("tags.name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ActivityID] = append(result[row.ActivityID], model.Tag{
			ID: row.ID, UUID: row.UUID, Name: row.Name, Slug: row.Slug,
			Color: row.Color, Icon: row.Icon, Status: row.Status,
		})
	}
	return result, nil
}

type activityTagRow struct {
	ActivityID uint64
	ID         uint64
	UUID       string
	Name       string
	Slug       string
	Color      string
	Icon       string
	Status     uint8
}

func replaceActivityTags(tx *gorm.DB, activityID uint64, tagIDs []uint64) error {
	if err := tx.Exec("DELETE FROM activity_tag_relations WHERE activity_id = ?", activityID).Error; err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		err := tx.Exec("INSERT IGNORE INTO activity_tag_relations (activity_id, tag_id, created_at) VALUES (?, ?, UTC_TIMESTAMP(3))", activityID, tagID).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// activityOrder maps the public sort parameter to a whitelisted ORDER BY.
func activityOrder(sort string) string {
	switch sort {
	case "created_at", "latest":
		return "activities.created_at DESC, activities.id DESC"
	case "hot", "view_count":
		return "activities.view_count DESC, activities.id DESC"
	default:
		return "activities.activity_start_at ASC, activities.id ASC"
	}
}

// nextTimedStatus returns the time based target status for a published or
// ongoing activity.
func nextTimedStatus(current model.ActivityStatus, startAt, endAt, now time.Time) (model.ActivityStatus, bool) {
	switch current {
	case model.ActivityPublished:
		if !endAt.After(now) {
			return model.ActivityFinished, true
		}
		if !startAt.After(now) {
			return model.ActivityOngoing, true
		}
	case model.ActivityOngoing:
		if !endAt.After(now) {
			return model.ActivityFinished, true
		}
	}
	return current, false
}
