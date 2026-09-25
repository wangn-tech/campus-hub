package service

import (
	"context"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
)

type ActivityView struct {
	ID                       string            `json:"id"`
	Title                    string            `json:"title"`
	CoverURL                 string            `json:"cover_url"`
	Category                 ActivityCategory  `json:"category"`
	Tags                     []ActivityTagView `json:"tags"`
	Organizer                ActivityOrganizer `json:"organizer"`
	Description              string            `json:"description"`
	ContactPhone             string            `json:"contact_phone"`
	RegisterStartAt          timestamp.Millis  `json:"register_start_at"`
	RegisterEndAt            timestamp.Millis  `json:"register_end_at"`
	ActivityStartAt          timestamp.Millis  `json:"activity_start_at"`
	ActivityEndAt            timestamp.Millis  `json:"activity_end_at"`
	Location                 string            `json:"location"`
	AddressDetail            string            `json:"address_detail"`
	Longitude                *float64          `json:"longitude"`
	Latitude                 *float64          `json:"latitude"`
	MaxParticipants          uint32            `json:"max_participants"`
	ApprovedParticipantCount uint32            `json:"approved_participant_count"`
	PendingParticipantCount  uint32            `json:"pending_participant_count"`
	RequireApproval          bool              `json:"require_approval"`
	RequireStudentVerify     bool              `json:"require_student_verify"`
	MinCreditScore           int               `json:"min_credit_score"`
	Status                   uint8             `json:"status"`
	RejectReason             string            `json:"reject_reason"`
	ViewCount                uint64            `json:"view_count"`
	Version                  uint32            `json:"version"`
	CreatedAt                timestamp.Millis  `json:"created_at"`
	UpdatedAt                timestamp.Millis  `json:"updated_at"`
}

type ActivityCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ActivityTagView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

type ActivityOrganizer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// buildViews assembles the API representation for a page of activities,
// batching the category, organizer avatar, cover and tag lookups.
func (s *ActivityService) buildViews(ctx context.Context, activities []model.Activity) ([]ActivityView, error) {
	views := make([]ActivityView, 0, len(activities))
	if len(activities) == 0 {
		return views, nil
	}
	activityIDs := make([]uint64, 0, len(activities))
	categoryIDs := make([]uint64, 0, len(activities))
	organizerIDs := make([]uint64, 0, len(activities))
	fileIDs := make([]uint64, 0, len(activities))
	for _, activity := range activities {
		activityIDs = append(activityIDs, activity.ID)
		categoryIDs = append(categoryIDs, activity.CategoryID)
		organizerIDs = append(organizerIDs, activity.OrganizerID)
		if activity.CoverFileID != nil {
			fileIDs = append(fileIDs, *activity.CoverFileID)
		}
	}
	tagMap, err := s.activities.TagsForActivityIDs(ctx, activityIDs)
	if err != nil {
		return nil, err
	}
	categories, err := s.categories.FindByIDs(ctx, uniqueUint64(categoryIDs))
	if err != nil {
		return nil, err
	}
	organizers, err := s.users.FindByIDs(ctx, uniqueUint64(organizerIDs))
	if err != nil {
		return nil, err
	}
	for _, organizer := range organizers {
		if organizer.AvatarFileID != nil {
			fileIDs = append(fileIDs, *organizer.AvatarFileID)
		}
	}
	files, err := s.files.FindByIDs(ctx, uniqueUint64(fileIDs))
	if err != nil {
		return nil, err
	}
	urls := make(map[uint64]string, len(files))
	for i := range files {
		url, err := s.attachments.AccessURL(ctx, &files[i])
		if err != nil {
			return nil, err
		}
		urls[files[i].ID] = url
	}
	categoryByID := make(map[uint64]model.Category, len(categories))
	for _, category := range categories {
		categoryByID[category.ID] = category
	}
	organizerByID := make(map[uint64]model.User, len(organizers))
	for _, organizer := range organizers {
		organizerByID[organizer.ID] = organizer
	}
	for _, activity := range activities {
		view := ActivityView{
			ID:                       activity.UUID,
			Title:                    activity.Title,
			Description:              activity.Description,
			ContactPhone:             activity.ContactPhone,
			RegisterStartAt:          timestamp.Millis(activity.RegisterStartAt),
			RegisterEndAt:            timestamp.Millis(activity.RegisterEndAt),
			ActivityStartAt:          timestamp.Millis(activity.ActivityStartAt),
			ActivityEndAt:            timestamp.Millis(activity.ActivityEndAt),
			Location:                 activity.Location,
			AddressDetail:            activity.AddressDetail,
			Longitude:                activity.Longitude,
			Latitude:                 activity.Latitude,
			MaxParticipants:          activity.MaxParticipants,
			ApprovedParticipantCount: activity.ApprovedParticipantCount,
			PendingParticipantCount:  activity.PendingParticipantCount,
			RequireApproval:          activity.RequireApproval,
			RequireStudentVerify:     activity.RequireStudentVerify,
			MinCreditScore:           activity.MinCreditScore,
			Status:                   activity.Status,
			RejectReason:             activity.RejectReason,
			ViewCount:                activity.ViewCount,
			Version:                  activity.Version,
			CreatedAt:                timestamp.Millis(activity.CreatedAt),
			UpdatedAt:                timestamp.Millis(activity.UpdatedAt),
		}
		if activity.CoverFileID != nil {
			view.CoverURL = urls[*activity.CoverFileID]
		}
		if category, ok := categoryByID[activity.CategoryID]; ok {
			view.Category = ActivityCategory{ID: category.UUID, Name: category.Name}
		}
		if organizer, ok := organizerByID[activity.OrganizerID]; ok {
			view.Organizer = ActivityOrganizer{ID: organizer.UUID, Name: organizer.Nickname}
			if organizer.AvatarFileID != nil {
				view.Organizer.AvatarURL = urls[*organizer.AvatarFileID]
			}
		} else {
			view.Organizer = ActivityOrganizer{Name: activity.OrganizerName}
		}
		for _, tag := range tagMap[activity.ID] {
			view.Tags = append(view.Tags, ActivityTagView{ID: tag.UUID, Name: tag.Name, Color: tag.Color, Icon: tag.Icon})
		}
		views = append(views, view)
	}
	return views, nil
}
