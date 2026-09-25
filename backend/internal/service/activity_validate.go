package service

import (
	"context"
	"strings"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
)

// validateForm checks the create/update payload and resolves the referenced
// category, tags and cover file.
func (s *ActivityService) validateForm(ctx context.Context, user *model.User, form ActivityForm) (*model.Category, []model.Tag, *model.File, error) {
	title := strings.TrimSpace(form.Title)
	if len([]rune(title)) < 1 || len([]rune(title)) > activityMaxTitleLen {
		return nil, nil, nil, ErrActivityInvalid
	}
	location := strings.TrimSpace(form.Location)
	if len([]rune(location)) < 1 || len([]rune(location)) > activityMaxLocLen {
		return nil, nil, nil, ErrActivityInvalid
	}
	if len([]rune(form.Description)) > activityMaxDescLen ||
		len([]rune(form.AddressDetail)) > activityMaxAddrLen ||
		len([]rune(strings.TrimSpace(form.ContactPhone))) > activityMaxPhoneLen {
		return nil, nil, nil, ErrActivityInvalid
	}
	if form.MaxParticipants < 1 || form.MinCreditScore < 0 {
		return nil, nil, nil, ErrActivityInvalid
	}
	if !validCoordinate(form.Longitude, 180) || !validCoordinate(form.Latitude, 90) {
		return nil, nil, nil, ErrActivityInvalid
	}
	if !validActivityWindow(form.RegisterStartAt.Time(), form.RegisterEndAt.Time(), form.ActivityStartAt.Time(), form.ActivityEndAt.Time()) {
		return nil, nil, nil, ErrActivityInvalid
	}
	category, err := s.categories.FindByUUID(ctx, strings.TrimSpace(form.CategoryID))
	if err != nil || category.Status != 1 {
		return nil, nil, nil, ErrActivityInvalid
	}
	tagUUIDs := uniqueStrings(form.TagIDs)
	tags, err := s.tags.FindByUUIDsForScope(ctx, tagUUIDs, "activity")
	if err != nil {
		return nil, nil, nil, err
	}
	if len(tags) != len(tagUUIDs) {
		return nil, nil, nil, ErrActivityInvalid
	}
	var cover *model.File
	if strings.TrimSpace(form.CoverFileID) != "" {
		file, err := s.files.FindByUUID(ctx, strings.TrimSpace(form.CoverFileID))
		if err != nil || file.UploaderID != user.ID || file.BizType != activityCoverBizType {
			return nil, nil, nil, ErrActivityInvalid
		}
		cover = file
	}
	return category, tags, cover, nil
}

func validCoordinate(value *float64, limit float64) bool {
	if value == nil {
		return true
	}
	return *value >= -limit && *value <= limit
}

// validActivityWindow enforces that the registration window is non-empty,
// closes no later than the activity starts, and that the activity window is
// non-empty.
func validActivityWindow(registerStart, registerEnd, activityStart, activityEnd time.Time) bool {
	if registerStart.IsZero() || registerEnd.IsZero() || activityStart.IsZero() || activityEnd.IsZero() {
		return false
	}
	return registerStart.Before(registerEnd) && activityStart.Before(activityEnd) && !registerEnd.After(activityStart)
}
