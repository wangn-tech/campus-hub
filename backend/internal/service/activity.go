package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrActivityNotFound = errors.New("activity not found")
	ErrActivityInvalid  = errors.New("activity invalid input")
	ErrActivityConflict = errors.New("activity state conflict")
)

const (
	activityCoverBizType = "activity_cover"
	activityRefType      = "activity"
	activityCoverPurpose = "cover"
	activityMaxTitleLen  = 100
	activityMaxDescLen   = 20000
	activityMaxLocLen    = 200
	activityMaxAddrLen   = 500
	activityMaxPhoneLen  = 20
	activityDefaultPage  = 10
	activityMaxPageSize  = 50
)

var (
	publicActivityStatuses = []int{
		int(model.ActivityPublished),
		int(model.ActivityOngoing),
		int(model.ActivityFinished),
	}
	allActivityStatuses = []int{0, 1, 2, 3, 4, 5, 6}
)

type ActivityService struct {
	activities  *repository.ActivityRepository
	categories  *repository.CategoryRepository
	tags        *repository.TagRepository
	users       *repository.UserRepository
	files       *repository.FileRepository
	attachments *FileService
}

func NewActivityService(activities *repository.ActivityRepository, categories *repository.CategoryRepository, tags *repository.TagRepository, users *repository.UserRepository, files *repository.FileRepository, attachments *FileService) *ActivityService {
	return &ActivityService{activities: activities, categories: categories, tags: tags, users: users, files: files, attachments: attachments}
}

// ActivityForm is the shared create/update payload. Times are Unix
// milliseconds and identifiers are UUID strings.
type ActivityForm struct {
	Title                string           `json:"title"`
	CoverFileID          string           `json:"cover_file_id"`
	Description          string           `json:"description"`
	CategoryID           string           `json:"category_id"`
	ContactPhone         string           `json:"contact_phone"`
	RegisterStartAt      timestamp.Millis `json:"register_start_at"`
	RegisterEndAt        timestamp.Millis `json:"register_end_at"`
	ActivityStartAt      timestamp.Millis `json:"activity_start_at"`
	ActivityEndAt        timestamp.Millis `json:"activity_end_at"`
	Location             string           `json:"location"`
	AddressDetail        string           `json:"address_detail"`
	Longitude            *float64         `json:"longitude"`
	Latitude             *float64         `json:"latitude"`
	MaxParticipants      uint32           `json:"max_participants"`
	RequireApproval      bool             `json:"require_approval"`
	RequireStudentVerify bool             `json:"require_student_verify"`
	MinCreditScore       int              `json:"min_credit_score"`
	TagIDs               []string         `json:"tag_ids"`
}

type ActivityCreateInput struct {
	ActivityForm
	IsDraft bool `json:"is_draft"`
}

type ActivityListQuery struct {
	CategoryID string
	Status     *int
	Sort       string
	Page       int
	PageSize   int
}

type ActivitySearchQuery struct {
	Keyword    string
	CategoryID string
	TagID      string
	StartTime  *int64
	EndTime    *int64
	Location   string
	Page       int
	PageSize   int
}

func (s *ActivityService) Categories(ctx context.Context) ([]model.Category, error) {
	return s.categories.ListActive(ctx)
}

// Tags lists enabled tags for a scope ("activity" or "interest").
func (s *ActivityService) Tags(ctx context.Context, scope string) ([]model.Tag, error) {
	if scope != "activity" {
		scope = "interest"
	}
	return s.tags.ListByScope(ctx, scope)
}

func (s *ActivityService) Create(ctx context.Context, user *model.User, in ActivityCreateInput, trace string) (*model.Activity, error) {
	category, tags, cover, err := s.validateForm(ctx, user, in.ActivityForm)
	if err != nil {
		return nil, err
	}
	activity := &model.Activity{
		UUID:                 uuid.NewString(),
		Title:                strings.TrimSpace(in.Title),
		Description:          strings.TrimSpace(in.Description),
		CategoryID:           category.ID,
		OrganizerID:          user.ID,
		OrganizerName:        user.Nickname,
		ContactPhone:         strings.TrimSpace(in.ContactPhone),
		RegisterStartAt:      in.RegisterStartAt.Time(),
		RegisterEndAt:        in.RegisterEndAt.Time(),
		ActivityStartAt:      in.ActivityStartAt.Time(),
		ActivityEndAt:        in.ActivityEndAt.Time(),
		Location:             strings.TrimSpace(in.Location),
		AddressDetail:        strings.TrimSpace(in.AddressDetail),
		Longitude:            in.Longitude,
		Latitude:             in.Latitude,
		MaxParticipants:      in.MaxParticipants,
		RequireApproval:      in.RequireApproval,
		RequireStudentVerify: in.RequireStudentVerify,
		MinCreditScore:       in.MinCreditScore,
		Status:               uint8(model.ActivityDraft),
	}
	if cover != nil {
		activity.CoverFileID = &cover.ID
	}
	if err := s.activities.Create(ctx, activity, tagIDList(tags)); err != nil {
		return nil, err
	}
	if cover != nil {
		_ = s.files.AddReference(ctx, cover.ID, activityRefType, activity.ID, activityCoverPurpose)
	}
	if !in.IsDraft {
		if err := s.transition(ctx, activity, uint8(model.ActivityPendingReview), model.ActivityOperatorUser, user.ID, "", trace); err != nil {
			return nil, err
		}
		activity.Status = uint8(model.ActivityPendingReview)
	}
	return activity, nil
}

func (s *ActivityService) Update(ctx context.Context, user *model.User, id string, form ActivityForm) (*model.Activity, error) {
	activity, err := s.findOwned(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if !isEditableActivityStatus(activity.Status) {
		return nil, ErrActivityConflict
	}
	category, tags, cover, err := s.validateForm(ctx, user, form)
	if err != nil {
		return nil, err
	}
	values := map[string]any{
		"title":                  strings.TrimSpace(form.Title),
		"description":            strings.TrimSpace(form.Description),
		"category_id":            category.ID,
		"contact_phone":          strings.TrimSpace(form.ContactPhone),
		"register_start_at":      form.RegisterStartAt.Time(),
		"register_end_at":        form.RegisterEndAt.Time(),
		"activity_start_at":      form.ActivityStartAt.Time(),
		"activity_end_at":        form.ActivityEndAt.Time(),
		"location":               strings.TrimSpace(form.Location),
		"address_detail":         strings.TrimSpace(form.AddressDetail),
		"longitude":              form.Longitude,
		"latitude":               form.Latitude,
		"max_participants":       form.MaxParticipants,
		"require_approval":       form.RequireApproval,
		"require_student_verify": form.RequireStudentVerify,
		"min_credit_score":       form.MinCreditScore,
		"cover_file_id":          nil,
	}
	if cover != nil {
		values["cover_file_id"] = cover.ID
	}
	if activity.CoverFileID != nil && (cover == nil || *activity.CoverFileID != cover.ID) {
		_ = s.files.RemoveReference(ctx, *activity.CoverFileID, activityRefType, activity.ID, activityCoverPurpose)
	}
	if cover != nil {
		_ = s.files.AddReference(ctx, cover.ID, activityRefType, activity.ID, activityCoverPurpose)
	}
	var log *model.ActivityStatusLog
	if model.ActivityStatus(activity.Status) == model.ActivityRejected {
		values["status"] = uint8(model.ActivityDraft)
		values["reject_reason"] = ""
		log = &model.ActivityStatusLog{
			UUID:         uuid.NewString(),
			ActivityID:   activity.ID,
			FromStatus:   uint8(model.ActivityRejected),
			ToStatus:     uint8(model.ActivityDraft),
			OperatorID:   user.ID,
			OperatorType: model.ActivityOperatorUser,
			Reason:       "edited after rejection",
			CreatedAt:    time.Now().UTC(),
		}
	}
	if err := s.activities.Update(ctx, activity, values, tagIDList(tags), log); err != nil {
		if errors.Is(err, repository.ErrConcurrentUpdate) {
			return nil, ErrActivityConflict
		}
		return nil, err
	}
	return s.activities.FindByUUID(ctx, activity.UUID)
}

func (s *ActivityService) Submit(ctx context.Context, user *model.User, id, trace string) error {
	activity, err := s.findOwned(ctx, user, id)
	if err != nil {
		return err
	}
	if !isEditableActivityStatus(activity.Status) {
		return ErrActivityConflict
	}
	return s.transition(ctx, activity, uint8(model.ActivityPendingReview), model.ActivityOperatorUser, user.ID, "", trace)
}

func (s *ActivityService) Cancel(ctx context.Context, user *model.User, id, trace string) error {
	activity, err := s.findOwned(ctx, user, id)
	if err != nil {
		return err
	}
	if !canCancelActivity(activity.Status) {
		return ErrActivityConflict
	}
	return s.transition(ctx, activity, uint8(model.ActivityCancelled), model.ActivityOperatorUser, user.ID, "", trace)
}

func (s *ActivityService) Approve(ctx context.Context, admin *model.User, id, trace string) error {
	return s.review(ctx, admin, id, uint8(model.ActivityPublished), "", trace)
}

func (s *ActivityService) Reject(ctx context.Context, admin *model.User, id, reason, trace string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 500 {
		return ErrActivityInvalid
	}
	return s.review(ctx, admin, id, uint8(model.ActivityRejected), reason, trace)
}

func (s *ActivityService) SendBack(ctx context.Context, admin *model.User, id, trace string) error {
	return s.review(ctx, admin, id, uint8(model.ActivityDraft), "", trace)
}

func (s *ActivityService) review(ctx context.Context, admin *model.User, id string, to uint8, reason, trace string) error {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrActivityNotFound
		}
		return err
	}
	if !canReviewActivity(activity.Status) {
		return ErrActivityConflict
	}
	return s.transition(ctx, activity, to, model.ActivityOperatorAdmin, admin.ID, reason, trace)
}

func (s *ActivityService) Detail(ctx context.Context, viewerUUID, id string) (*ActivityView, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActivityNotFound
		}
		return nil, err
	}
	if !isPublicActivityStatus(activity.Status) {
		visible, err := s.canViewPrivate(ctx, activity, viewerUUID)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, ErrActivityNotFound
		}
	}
	if err := s.activities.IncrementViewCount(ctx, activity.ID); err != nil {
		return nil, err
	}
	activity.ViewCount++
	views, err := s.buildViews(ctx, []model.Activity{*activity})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *ActivityService) List(ctx context.Context, query ActivityListQuery) ([]ActivityView, int64, error) {
	page, pageSize := NormalizePage(query.Page, query.PageSize)
	statuses, ok := requestedPublicStatuses(query.Status)
	if !ok {
		return []ActivityView{}, 0, nil
	}
	filter := repository.ActivityFilter{Statuses: statuses, Sort: query.Sort, Page: page, PageSize: pageSize}
	if strings.TrimSpace(query.CategoryID) != "" {
		category, err := s.categories.FindByUUID(ctx, strings.TrimSpace(query.CategoryID))
		if err != nil {
			return []ActivityView{}, 0, nil
		}
		filter.CategoryID = &category.ID
	}
	return s.listViews(ctx, filter)
}

func (s *ActivityService) Search(ctx context.Context, query ActivitySearchQuery) ([]ActivityView, int64, error) {
	page, pageSize := NormalizePage(query.Page, query.PageSize)
	filter := repository.ActivityFilter{
		Statuses: publicActivityStatuses,
		Keyword:  strings.TrimSpace(query.Keyword),
		Location: strings.TrimSpace(query.Location),
		Page:     page,
		PageSize: pageSize,
	}
	if strings.TrimSpace(query.CategoryID) != "" {
		category, err := s.categories.FindByUUID(ctx, strings.TrimSpace(query.CategoryID))
		if err != nil {
			return []ActivityView{}, 0, nil
		}
		filter.CategoryID = &category.ID
	}
	if strings.TrimSpace(query.TagID) != "" {
		tags, err := s.tags.FindByUUIDsForScope(ctx, []string{strings.TrimSpace(query.TagID)}, "activity")
		if err != nil || len(tags) == 0 {
			return []ActivityView{}, 0, nil
		}
		filter.TagID = &tags[0].ID
	}
	if query.StartTime != nil {
		at := timestamp.FromMillis(*query.StartTime)
		filter.StartFrom = &at
	}
	if query.EndTime != nil {
		at := timestamp.FromMillis(*query.EndTime)
		filter.StartTo = &at
	}
	return s.listViews(ctx, filter)
}

func (s *ActivityService) MyCreated(ctx context.Context, user *model.User, page, pageSize int) ([]ActivityView, int64, error) {
	normalizedPage, normalizedSize := NormalizePage(page, pageSize)
	filter := repository.ActivityFilter{
		Statuses:    allActivityStatuses,
		OrganizerID: &user.ID,
		Page:        normalizedPage,
		PageSize:    normalizedSize,
	}
	return s.listViews(ctx, filter)
}

// SyncStatuses applies the time based published -> ongoing -> finished flow.
func (s *ActivityService) SyncStatuses(ctx context.Context) (int64, error) {
	return s.activities.SyncStatuses(ctx, time.Now().UTC())
}

func (s *ActivityService) listViews(ctx context.Context, filter repository.ActivityFilter) ([]ActivityView, int64, error) {
	items, total, err := s.activities.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.buildViews(ctx, items)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *ActivityService) findOwned(ctx context.Context, user *model.User, id string) (*model.Activity, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActivityNotFound
		}
		return nil, err
	}
	if activity.OrganizerID != user.ID {
		return nil, ErrForbidden
	}
	return activity, nil
}

func (s *ActivityService) transition(ctx context.Context, activity *model.Activity, to, operatorType uint8, operatorID uint64, reason, trace string) error {
	now := time.Now().UTC()
	values := map[string]any{
		"status":  to,
		"version": gorm.Expr("version + 1"),
	}
	switch model.ActivityStatus(to) {
	case model.ActivityPublished:
		values["published_at"] = now
		values["reject_reason"] = ""
	case model.ActivityRejected:
		values["reject_reason"] = reason
	case model.ActivityDraft:
		values["reject_reason"] = ""
	}
	log := &model.ActivityStatusLog{
		UUID:         uuid.NewString(),
		ActivityID:   activity.ID,
		FromStatus:   activity.Status,
		ToStatus:     to,
		OperatorID:   operatorID,
		OperatorType: operatorType,
		Reason:       reason,
		TraceID:      trace,
		CreatedAt:    now,
	}
	if err := s.activities.Transition(ctx, activity.ID, activity.Status, values, log); err != nil {
		if errors.Is(err, repository.ErrConcurrentUpdate) {
			return ErrActivityConflict
		}
		return err
	}
	activity.Status = to
	return nil
}

func (s *ActivityService) canViewPrivate(ctx context.Context, activity *model.Activity, viewerUUID string) (bool, error) {
	viewerUUID = strings.TrimSpace(viewerUUID)
	if viewerUUID == "" {
		return false, nil
	}
	viewer, err := s.users.FindByUUID(ctx, viewerUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if viewer.ID == activity.OrganizerID {
		return true, nil
	}
	return s.users.HasRole(ctx, viewer.ID, model.RoleAdmin)
}

// isEditableActivityStatus reports whether the organizer may still edit the
// activity. Rejected activities return to draft when edited.
func isEditableActivityStatus(status uint8) bool {
	return status == uint8(model.ActivityDraft) || status == uint8(model.ActivityRejected)
}

// canCancelActivity reports whether the organizer may cancel the activity.
func canCancelActivity(status uint8) bool {
	switch model.ActivityStatus(status) {
	case model.ActivityDraft, model.ActivityPendingReview, model.ActivityPublished, model.ActivityOngoing:
		return true
	default:
		return false
	}
}

// canReviewActivity reports whether an administrator may approve, reject or
// send back the activity.
func canReviewActivity(status uint8) bool {
	return status == uint8(model.ActivityPendingReview)
}

func isPublicActivityStatus(status uint8) bool {
	for _, allowed := range publicActivityStatuses {
		if int(status) == allowed {
			return true
		}
	}
	return false
}

// requestedPublicStatuses translates the public status filter. The second
// return value is false when a non public status was requested.
func requestedPublicStatuses(status *int) ([]int, bool) {
	if status == nil {
		return publicActivityStatuses, true
	}
	for _, allowed := range publicActivityStatuses {
		if *status == allowed {
			return []int{*status}, true
		}
	}
	return nil, false
}

// NormalizePage clamps the requested page and page_size to the API defaults.
func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = activityDefaultPage
	}
	if pageSize > activityMaxPageSize {
		pageSize = activityMaxPageSize
	}
	return page, pageSize
}

func tagIDList(tags []model.Tag) []uint64 {
	ids := make([]uint64, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
	}
	return ids
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueUint64(values []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
