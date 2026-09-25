package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrRegistrationNotFound    = errors.New("registration not found")
	ErrRegistrationConflict    = errors.New("registration conflict")
	ErrRegistrationNotEligible = errors.New("registration requirements not met")
	ErrRegistrationInvalid     = errors.New("registration invalid input")
	ErrTicketNotFound          = errors.New("ticket not found")
)

const (
	// registrationApprovalWindow bounds how long a pending registration keeps
	// its reserved slot.
	registrationApprovalWindow = 48 * time.Hour
	// ticketValidLeadTime opens the check-in window before the activity starts.
	ticketValidLeadTime = time.Hour
	// registrationActiveFlag marks pending/approved rows, which the
	// uk_active_registration unique key uses to allow one active row per user.
	registrationActiveFlag uint8 = 1
)

type RegistrationService struct {
	registrations *repository.RegistrationRepository
	tickets       *repository.TicketRepository
	activities    *repository.ActivityRepository
	users         *repository.UserRepository
	files         *repository.FileRepository
	verifications *repository.VerificationRepository
	attachments   *FileService
}

func NewRegistrationService(registrations *repository.RegistrationRepository, tickets *repository.TicketRepository, activities *repository.ActivityRepository, users *repository.UserRepository, files *repository.FileRepository, verifications *repository.VerificationRepository, attachments *FileService) *RegistrationService {
	return &RegistrationService{
		registrations: registrations,
		tickets:       tickets,
		activities:    activities,
		users:         users,
		files:         files,
		verifications: verifications,
		attachments:   attachments,
	}
}

// Register creates the caller's registration, reserving a slot and issuing the
// ticket immediately when the activity does not require approval.
func (s *RegistrationService) Register(ctx context.Context, user *model.User, activityUUID, trace string) (*RegistrationView, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(activityUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrActivityNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	if !isRegisterableActivityStatus(activity.Status) || now.Before(activity.RegisterStartAt) || now.After(activity.RegisterEndAt) {
		return nil, ErrRegistrationConflict
	}
	if err := s.checkEligibility(ctx, user, activity); err != nil {
		return nil, err
	}
	if _, err := s.registrations.FindActive(ctx, activity.ID, user.ID); err == nil {
		return nil, ErrRegistrationConflict
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	approved := !activity.RequireApproval
	activeFlag := registrationActiveFlag
	registration := &model.Registration{
		UUID:       uuid.NewString(),
		ActivityID: activity.ID,
		UserID:     user.ID,
		Status:     uint8(model.RegistrationPending),
		ActiveFlag: &activeFlag,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if approved {
		registration.Status = uint8(model.RegistrationApproved)
	} else {
		expiresAt := approvalDeadline(now, activity.ActivityStartAt)
		registration.ExpiresAt = &expiresAt
	}

	created, err := newOutboxEvent("registration.created", "registration", registration.UUID, trace, map[string]any{
		"activity_id": activity.UUID,
		"user_id":     user.UUID,
		"status":      registration.Status,
	})
	if err != nil {
		return nil, err
	}
	events := []*model.OutboxEvent{created}

	var ticket *model.Ticket
	if approved {
		ticket, err = s.issueTicket(ctx, activity, registration)
		if err != nil {
			return nil, err
		}
		issued, err := newOutboxEvent("ticket.created", "ticket", ticket.UUID, trace, map[string]any{
			"activity_id":     activity.UUID,
			"registration_id": registration.UUID,
			"user_id":         user.UUID,
		})
		if err != nil {
			return nil, err
		}
		events = append(events, issued)
	}

	input := repository.RegisterInput{Registration: registration, Ticket: ticket, Events: events, Approved: approved}
	if err := s.registrations.Register(ctx, input); err != nil {
		if errors.Is(err, repository.ErrCapacityFull) {
			return nil, ErrRegistrationConflict
		}
		return nil, err
	}
	views, err := s.buildRegistrationViews(ctx, []model.Registration{*registration}, false)
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// Cancel releases the caller's own registration and voids its ticket.
func (s *RegistrationService) Cancel(ctx context.Context, user *model.User, registrationUUID, trace string) error {
	registration, err := s.registrations.FindByUUID(ctx, strings.TrimSpace(registrationUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRegistrationNotFound
		}
		return err
	}
	if registration.UserID != user.ID {
		return ErrForbidden
	}
	if !canCancelRegistration(registration.Status) {
		return ErrRegistrationConflict
	}
	activity, err := s.activities.FindByID(ctx, registration.ActivityID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	pendingDelta, approvedDelta := 0, 0
	if model.RegistrationStatus(registration.Status) == model.RegistrationApproved {
		approvedDelta = -1
	} else {
		pendingDelta = -1
	}
	input := repository.TransitionInput{
		RegistrationID: registration.ID,
		ActivityID:     registration.ActivityID,
		FromStatus:     registration.Status,
		Values: map[string]any{
			"status":      uint8(model.RegistrationCancelled),
			"active_flag": nil,
			"cancel_time": now,
			"updated_at":  now,
		},
		PendingDelta:  pendingDelta,
		ApprovedDelta: approvedDelta,
		Log: &model.RegistrationStatusLog{
			RegistrationID: registration.ID,
			FromStatus:     registration.Status,
			ToStatus:       uint8(model.RegistrationCancelled),
			OperatorID:     user.ID,
			OperatorType:   model.RegistrationOperatorUser,
			CreatedAt:      now,
		},
	}
	cancelled, err := newOutboxEvent("registration.cancelled", "registration", registration.UUID, trace, map[string]any{
		"activity_id": activity.UUID,
		"user_id":     user.UUID,
	})
	if err != nil {
		return err
	}
	input.Events = append(input.Events, cancelled)

	ticket, err := s.tickets.FindByRegistrationID(ctx, registration.ID)
	switch {
	case err == nil:
		if model.TicketStatus(ticket.Status) == model.TicketUnused {
			input.TicketID = &ticket.ID
			voided, err := newOutboxEvent("ticket.voided", "ticket", ticket.UUID, trace, map[string]any{
				"activity_id":     activity.UUID,
				"registration_id": registration.UUID,
			})
			if err != nil {
				return err
			}
			input.Events = append(input.Events, voided)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
	default:
		return err
	}

	return s.applyTransition(ctx, input)
}

// Detail returns one registration to the registrant, the activity organizer or
// an administrator.
func (s *RegistrationService) Detail(ctx context.Context, viewer *model.User, registrationUUID string) (*RegistrationView, error) {
	registration, err := s.registrations.FindByUUID(ctx, strings.TrimSpace(registrationUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRegistrationNotFound
		}
		return nil, err
	}
	activity, err := s.activities.FindByID(ctx, registration.ActivityID)
	if err != nil {
		return nil, err
	}
	withUser := registration.UserID != viewer.ID
	if withUser {
		managed, err := s.canManage(ctx, activity, viewer)
		if err != nil {
			return nil, err
		}
		if !managed {
			return nil, ErrForbidden
		}
	}
	views, err := s.buildRegistrationViews(ctx, []model.Registration{*registration}, withUser)
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// MyRegistrations lists the caller's registrations. The optional status filter
// is upcoming (activity has not started) or history (it has).
func (s *RegistrationService) MyRegistrations(ctx context.Context, user *model.User, status string, page, pageSize int) ([]RegistrationView, int64, error) {
	page, pageSize = NormalizePage(page, pageSize)
	filter := repository.UserRegistrationFilter{UserID: user.ID, Page: page, PageSize: pageSize}
	switch strings.TrimSpace(status) {
	case "upcoming":
		upcoming := true
		filter.Upcoming = &upcoming
	case "history":
		upcoming := false
		filter.Upcoming = &upcoming
	}
	registrations, total, err := s.registrations.ListByUser(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.buildRegistrationViews(ctx, registrations, false)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *RegistrationService) MyTickets(ctx context.Context, user *model.User, page, pageSize int) ([]TicketView, int64, error) {
	page, pageSize = NormalizePage(page, pageSize)
	tickets, total, err := s.tickets.ListByUser(ctx, user.ID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.buildTicketViews(ctx, tickets)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *RegistrationService) Ticket(ctx context.Context, viewer *model.User, ticketUUID string) (*TicketView, error) {
	ticket, err := s.tickets.FindByUUID(ctx, strings.TrimSpace(ticketUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.UserID != viewer.ID {
		activity, err := s.activities.FindByID(ctx, ticket.ActivityID)
		if err != nil {
			return nil, err
		}
		managed, err := s.canManage(ctx, activity, viewer)
		if err != nil {
			return nil, err
		}
		if !managed {
			return nil, ErrForbidden
		}
	}
	views, err := s.buildTicketViews(ctx, []model.Ticket{*ticket})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// issueTicket creates the ticket row for an approved registration. The caller
// writes it inside the same transaction as the registration.
func (s *RegistrationService) issueTicket(ctx context.Context, activity *model.Activity, registration *model.Registration) (*model.Ticket, error) {
	code, err := s.tickets.NextCode(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	validStart := activity.ActivityStartAt.Add(-ticketValidLeadTime)
	return &model.Ticket{
		UUID:         uuid.NewString(),
		Code:         code,
		ActivityID:   activity.ID,
		UserID:       registration.UserID,
		Status:       uint8(model.TicketUnused),
		ValidStartAt: &validStart,
		ValidEndAt:   &activity.ActivityEndAt,
		IssuedAt:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// checkEligibility applies the activity's registration requirements.
func (s *RegistrationService) checkEligibility(ctx context.Context, user *model.User, activity *model.Activity) error {
	if activity.RequireStudentVerify {
		verification, err := s.verifications.Current(ctx, user.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRegistrationNotEligible
		}
		if err != nil {
			return err
		}
		if model.StudentVerificationStatus(verification.Status) != model.VerificationApproved {
			return ErrRegistrationNotEligible
		}
	}
	if activity.MinCreditScore > 0 {
		score, err := s.users.CreditScore(ctx, user.ID)
		if err != nil {
			return err
		}
		if score < activity.MinCreditScore {
			return ErrRegistrationNotEligible
		}
	}
	return nil
}

// canManage reports whether the user owns the activity or is an administrator.
func (s *RegistrationService) canManage(ctx context.Context, activity *model.Activity, user *model.User) (bool, error) {
	return canManageActivity(ctx, s.users, activity, user)
}

// canManageActivity reports whether the user owns the activity or is an
// administrator.
func canManageActivity(ctx context.Context, users *repository.UserRepository, activity *model.Activity, user *model.User) (bool, error) {
	if activity.OrganizerID == user.ID {
		return true, nil
	}
	return users.HasRole(ctx, user.ID, model.RoleAdmin)
}

// approvalDeadline caps the approval window so a pending registration never
// outlives the activity it belongs to.
func approvalDeadline(now, activityStart time.Time) time.Time {
	deadline := now.Add(registrationApprovalWindow)
	if deadline.After(activityStart) {
		return activityStart
	}
	return deadline
}

// isRegisterableActivityStatus reports whether a user may register: the
// activity has to be published or already running.
func isRegisterableActivityStatus(status uint8) bool {
	switch model.ActivityStatus(status) {
	case model.ActivityPublished, model.ActivityOngoing:
		return true
	default:
		return false
	}
}

// canCancelRegistration reports whether the registration is still cancellable.
func canCancelRegistration(status uint8) bool {
	switch model.RegistrationStatus(status) {
	case model.RegistrationPending, model.RegistrationApproved:
		return true
	default:
		return false
	}
}
