package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"gorm.io/gorm"
)

// maxRejectReasonLen matches the reject_reason column.
const maxRejectReasonLen = 255

// Approve accepts a pending registration and issues its ticket.
func (s *RegistrationService) Approve(ctx context.Context, actor *model.User, registrationUUID, trace string) error {
	registration, activity, err := s.loadManaged(ctx, actor, registrationUUID)
	if err != nil {
		return err
	}
	if model.RegistrationStatus(registration.Status) != model.RegistrationPending {
		return ErrRegistrationConflict
	}
	ticket, err := s.issueTicket(ctx, activity, registration)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	events, err := s.reviewEvents(ctx, registration, activity, "registration.approved", uint8(model.RegistrationApproved), trace)
	if err != nil {
		return err
	}
	issued, err := newOutboxEvent("ticket.created", "ticket", ticket.UUID, trace, map[string]any{
		"activity_id":     activity.UUID,
		"registration_id": registration.UUID,
	})
	if err != nil {
		return err
	}
	events = append(events, issued)
	return s.applyTransition(ctx, repository.TransitionInput{
		RegistrationID: registration.ID,
		ActivityID:     registration.ActivityID,
		FromStatus:     registration.Status,
		Values: map[string]any{
			"status":      uint8(model.RegistrationApproved),
			"reviewed_by": actor.ID,
			"decided_at":  now,
			"updated_at":  now,
		},
		// The approval moves an already reserved slot, so capacity is not checked
		// again: the pending counter is released as the approved counter grows.
		PendingDelta:  -1,
		ApprovedDelta: 1,
		Ticket:        ticket,
		Log: &model.RegistrationStatusLog{
			RegistrationID: registration.ID,
			FromStatus:     registration.Status,
			ToStatus:       uint8(model.RegistrationApproved),
			OperatorID:     actor.ID,
			OperatorType:   reviewOperatorType(activity, actor),
			CreatedAt:      now,
		},
		Events: events,
	})
}

// Reject turns down a pending registration, releasing its reserved slot.
func (s *RegistrationService) Reject(ctx context.Context, actor *model.User, registrationUUID, reason, trace string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > maxRejectReasonLen {
		return ErrRegistrationInvalid
	}
	registration, activity, err := s.loadManaged(ctx, actor, registrationUUID)
	if err != nil {
		return err
	}
	if model.RegistrationStatus(registration.Status) != model.RegistrationPending {
		return ErrRegistrationConflict
	}
	now := time.Now().UTC()
	events, err := s.reviewEvents(ctx, registration, activity, "registration.rejected", uint8(model.RegistrationRejected), trace)
	if err != nil {
		return err
	}
	return s.applyTransition(ctx, repository.TransitionInput{
		RegistrationID: registration.ID,
		ActivityID:     registration.ActivityID,
		FromStatus:     registration.Status,
		Values: map[string]any{
			"status":        uint8(model.RegistrationRejected),
			"active_flag":   nil,
			"reject_reason": reason,
			"reviewed_by":   actor.ID,
			"decided_at":    now,
			"updated_at":    now,
		},
		PendingDelta: -1,
		Log: &model.RegistrationStatusLog{
			RegistrationID: registration.ID,
			FromStatus:     registration.Status,
			ToStatus:       uint8(model.RegistrationRejected),
			OperatorID:     actor.ID,
			OperatorType:   reviewOperatorType(activity, actor),
			Reason:         reason,
			CreatedAt:      now,
		},
		Events: events,
	})
}

// ListByActivity lists one activity's registrations for its organizer.
func (s *RegistrationService) ListByActivity(ctx context.Context, actor *model.User, activityUUID, status string, page, pageSize int) ([]RegistrationView, int64, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(activityUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrActivityNotFound
		}
		return nil, 0, err
	}
	managed, err := s.canManage(ctx, activity, actor)
	if err != nil {
		return nil, 0, err
	}
	if !managed {
		return nil, 0, ErrForbidden
	}
	statuses, ok := registrationStatusFilter(status)
	if !ok {
		return nil, 0, ErrRegistrationInvalid
	}
	page, pageSize = NormalizePage(page, pageSize)
	registrations, total, err := s.registrations.ListByActivity(ctx, activity.ID, statuses, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.buildRegistrationViews(ctx, registrations, true)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// ExpirePending releases the slot of every pending registration whose approval
// window has passed.
func (s *RegistrationService) ExpirePending(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	candidates, err := s.registrations.ListExpiredPending(ctx, now, 200)
	if err != nil {
		return 0, err
	}
	var expired int64
	for i := range candidates {
		registration := candidates[i]
		activity, err := s.activities.FindByID(ctx, registration.ActivityID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return expired, err
		}
		events, err := s.reviewEvents(ctx, &registration, activity, "registration.expired", uint8(model.RegistrationExpired), "")
		if err != nil {
			return expired, err
		}
		input := repository.TransitionInput{
			RegistrationID: registration.ID,
			ActivityID:     registration.ActivityID,
			FromStatus:     registration.Status,
			Values: map[string]any{
				"status":      uint8(model.RegistrationExpired),
				"active_flag": nil,
				"updated_at":  now,
			},
			PendingDelta: -1,
			Log: &model.RegistrationStatusLog{
				RegistrationID: registration.ID,
				FromStatus:     registration.Status,
				ToStatus:       uint8(model.RegistrationExpired),
				OperatorType:   model.RegistrationOperatorSystem,
				Reason:         "approval timeout",
				CreatedAt:      now,
			},
			Events: events,
		}
		if err := s.registrations.Transition(ctx, input); err != nil {
			if errors.Is(err, repository.ErrConcurrentUpdate) {
				continue
			}
			return expired, err
		}
		expired++
	}
	return expired, nil
}

// loadManaged resolves a registration and the activity it belongs to, and
// requires the actor to be the organizer or an administrator.
func (s *RegistrationService) loadManaged(ctx context.Context, actor *model.User, registrationUUID string) (*model.Registration, *model.Activity, error) {
	registration, err := s.registrations.FindByUUID(ctx, strings.TrimSpace(registrationUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrRegistrationNotFound
		}
		return nil, nil, err
	}
	activity, err := s.activities.FindByID(ctx, registration.ActivityID)
	if err != nil {
		return nil, nil, err
	}
	managed, err := s.canManage(ctx, activity, actor)
	if err != nil {
		return nil, nil, err
	}
	if !managed {
		return nil, nil, ErrForbidden
	}
	return registration, activity, nil
}

// reviewEvents builds the registration event for a review transition.
func (s *RegistrationService) reviewEvents(ctx context.Context, registration *model.Registration, activity *model.Activity, eventType string, status uint8, trace string) ([]*model.OutboxEvent, error) {
	payload := map[string]any{
		"activity_id":     activity.UUID,
		"registration_id": registration.UUID,
		"from_status":     registration.Status,
		"to_status":       status,
		"status":          status,
	}
	users, err := s.users.FindByIDs(ctx, []uint64{registration.UserID})
	if err != nil {
		return nil, err
	}
	if len(users) > 0 {
		payload["user_id"] = users[0].UUID
	}
	event, err := newOutboxEvent(eventType, "registration", registration.UUID, trace, payload)
	if err != nil {
		return nil, err
	}
	return []*model.OutboxEvent{event}, nil
}

func (s *RegistrationService) applyTransition(ctx context.Context, input repository.TransitionInput) error {
	if err := s.registrations.Transition(ctx, input); err != nil {
		if errors.Is(err, repository.ErrConcurrentUpdate) || errors.Is(err, repository.ErrCapacityFull) {
			return ErrRegistrationConflict
		}
		return err
	}
	return nil
}

// reviewOperatorType records whether the organizer or an administrator acted.
func reviewOperatorType(activity *model.Activity, actor *model.User) uint8 {
	if activity.OrganizerID == actor.ID {
		return model.RegistrationOperatorOrganizer
	}
	return model.RegistrationOperatorAdmin
}

// registrationStatusFilter maps the public status name to stored values. An
// empty filter means "all statuses".
func registrationStatusFilter(status string) ([]int, bool) {
	switch strings.TrimSpace(status) {
	case "":
		return nil, true
	case "pending":
		return []int{int(model.RegistrationPending)}, true
	case "approved":
		return []int{int(model.RegistrationApproved)}, true
	case "rejected":
		return []int{int(model.RegistrationRejected)}, true
	case "cancelled":
		return []int{int(model.RegistrationCancelled)}, true
	case "failed":
		return []int{int(model.RegistrationFailed)}, true
	case "expired":
		return []int{int(model.RegistrationExpired)}, true
	default:
		return nil, false
	}
}
