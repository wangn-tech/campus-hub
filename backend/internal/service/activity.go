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
	ErrActivityNotFound  = errors.New("activity not found")
	ErrActivityForbidden = errors.New("activity forbidden")
	ErrActivityInvalid   = errors.New("activity invalid input")
	ErrActivityConflict  = errors.New("activity state conflict")
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

func (s *ActivityService) Create(ctx context.Context, user *model.User, in ActivityCreateInput) (*model.Activity, error) {
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
		if err := s.transition(ctx, activity, uint8(model.ActivityPendingReview), model.ActivityOperatorUser, user.ID, "", ""); err != nil {
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
	if activity.Status != uint8(model.ActivityDraft) && activity.Status != uint8(model.ActivityRejected) {
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
	if activity.Status == uint8(model.ActivityRejected) {
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
	if activity.Status != uint8(model.ActivityDraft) && activity.Status != uint8(model.ActivityRejected) {
		return ErrActivityConflict
	}
	return s.transition(ctx, activity, uint8(model.ActivityPendingReview), model.ActivityOperatorUser, user.ID, "", trace)
}

func (s *ActivityService) Cancel(ctx context.Context, user *model.User, id, trace string) error {
	activity, err := s.findOwned(ctx, user, id)
	if err != nil {
		return err
	}
	switch model.ActivityStatus(activity.Status) {
	case model.ActivityDraft, model.ActivityPendingReview, model.ActivityPublished, model.ActivityOngoing:
	default:
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
	if activity.Status != uint8(model.ActivityPendingReview) {
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

func (s *ActivityService) findOwned(ctx context.Context, user *model.User, id string) (*model.Activity, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActivityNotFound
		}
		return nil, err
	}
	if activity.OrganizerID != user.ID {
		return nil, ErrActivityForbidden
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
