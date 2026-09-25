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
	ErrCheckInConflict = errors.New("check-in conflict")
	ErrCheckInInvalid  = errors.New("check-in invalid input")
)

// maxClientRequestIDLen matches the check_ins.client_request_id column.
const maxClientRequestIDLen = 64

type CheckInService struct {
	checkIns   *repository.CheckInRepository
	tickets    *repository.TicketRepository
	activities *repository.ActivityRepository
	users      *repository.UserRepository
}

func NewCheckInService(checkIns *repository.CheckInRepository, tickets *repository.TicketRepository, activities *repository.ActivityRepository, users *repository.UserRepository) *CheckInService {
	return &CheckInService{checkIns: checkIns, tickets: tickets, activities: activities, users: users}
}

// CheckInInput accepts either a ticket code or a ticket UUID, plus the client
// supplied idempotency key.
type CheckInInput struct {
	Code            string   `json:"code"`
	TicketID        string   `json:"ticket_id"`
	ActivityID      string   `json:"activity_id"`
	Longitude       *float64 `json:"longitude"`
	Latitude        *float64 `json:"latitude"`
	ClientRequestID string   `json:"client_request_id"`
}

// CheckIn redeems a ticket on behalf of the activity organizer or an
// administrator.
func (s *CheckInService) CheckIn(ctx context.Context, actor *model.User, in CheckInInput, trace string) (*CheckInView, error) {
	requestID := strings.TrimSpace(in.ClientRequestID)
	if requestID == "" || len(requestID) > maxClientRequestIDLen {
		return nil, ErrCheckInInvalid
	}
	code := strings.TrimSpace(in.Code)
	ticketUUID := strings.TrimSpace(in.TicketID)
	if (code == "") == (ticketUUID == "") {
		return nil, ErrCheckInInvalid
	}
	// Idempotent replay: the same client request id returns the first result.
	if existing, err := s.checkIns.FindByRequestID(ctx, requestID); err == nil {
		return s.viewFor(ctx, *existing)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	ticket, err := s.resolveTicket(ctx, code, ticketUUID)
	if err != nil {
		return nil, err
	}
	activity, err := s.activities.FindByID(ctx, ticket.ActivityID)
	if err != nil {
		return nil, err
	}
	if requested := strings.TrimSpace(in.ActivityID); requested != "" && requested != activity.UUID {
		return nil, ErrCheckInConflict
	}
	managed, err := canManageActivity(ctx, s.users, activity, actor)
	if err != nil {
		return nil, err
	}
	if !managed {
		return nil, ErrForbidden
	}
	now := time.Now().UTC()
	if err := checkInAllowed(ticket, activity, now); err != nil {
		return nil, err
	}
	record := &model.CheckIn{
		UUID:            uuid.NewString(),
		TicketID:        ticket.ID,
		RegistrationID:  ticket.RegistrationID,
		ActivityID:      activity.ID,
		UserID:          ticket.UserID,
		OperatorID:      actor.ID,
		ClientRequestID: requestID,
		Longitude:       in.Longitude,
		Latitude:        in.Latitude,
		CheckedInAt:     now,
		CreatedAt:       now,
	}
	event, err := newOutboxEvent("ticket.used", "ticket", ticket.UUID, trace, map[string]any{
		"activity_id": activity.UUID,
	})
	if err != nil {
		return nil, err
	}
	if err := s.checkIns.CheckIn(ctx, record, []*model.OutboxEvent{event}); err != nil {
		// A concurrent replay of the same request returns the winning record.
		if existing, findErr := s.checkIns.FindByRequestID(ctx, requestID); findErr == nil {
			return s.viewFor(ctx, *existing)
		}
		if errors.Is(err, repository.ErrTicketNotUsable) {
			return nil, ErrCheckInConflict
		}
		return nil, err
	}
	return s.viewFor(ctx, *record)
}

// List returns the check-in records of one activity for its organizer.
func (s *CheckInService) List(ctx context.Context, actor *model.User, activityUUID string, page, pageSize int) ([]CheckInView, int64, error) {
	activity, err := s.activities.FindByUUID(ctx, strings.TrimSpace(activityUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrActivityNotFound
		}
		return nil, 0, err
	}
	managed, err := canManageActivity(ctx, s.users, activity, actor)
	if err != nil {
		return nil, 0, err
	}
	if !managed {
		return nil, 0, ErrForbidden
	}
	page, pageSize = NormalizePage(page, pageSize)
	records, total, err := s.checkIns.ListByActivity(ctx, activity.ID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.buildCheckInViews(ctx, records)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// ExpireTickets closes the unused tickets whose validity window has passed.
func (s *CheckInService) ExpireTickets(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	tickets, err := s.tickets.ListExpiredUnused(ctx, now, 200)
	if err != nil {
		return 0, err
	}
	if len(tickets) == 0 {
		return 0, nil
	}
	activityIDs := make([]uint64, 0, len(tickets))
	for _, ticket := range tickets {
		activityIDs = append(activityIDs, ticket.ActivityID)
	}
	activities, err := s.activities.FindByIDs(ctx, uniqueUint64(activityIDs))
	if err != nil {
		return 0, err
	}
	activityUUID := make(map[uint64]string, len(activities))
	for _, activity := range activities {
		activityUUID[activity.ID] = activity.UUID
	}
	var expired int64
	for _, ticket := range tickets {
		event, err := newOutboxEvent("ticket.expired", "ticket", ticket.UUID, "", map[string]any{
			"activity_id": activityUUID[ticket.ActivityID],
		})
		if err != nil {
			return expired, err
		}
		if err := s.tickets.Expire(ctx, ticket.ID, []*model.OutboxEvent{event}); err != nil {
			if errors.Is(err, repository.ErrConcurrentUpdate) {
				continue
			}
			return expired, err
		}
		expired++
	}
	return expired, nil
}

func (s *CheckInService) resolveTicket(ctx context.Context, code, ticketUUID string) (*model.Ticket, error) {
	var (
		ticket *model.Ticket
		err    error
	)
	if ticketUUID != "" {
		ticket, err = s.tickets.FindByUUID(ctx, ticketUUID)
	} else {
		ticket, err = s.tickets.FindByCode(ctx, strings.ToUpper(code))
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return ticket, nil
}

func (s *CheckInService) viewFor(ctx context.Context, record model.CheckIn) (*CheckInView, error) {
	views, err := s.buildCheckInViews(ctx, []model.CheckIn{record})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// checkInAllowed reports whether the ticket may be redeemed right now: it has
// to be unused, the activity must not be cancelled, and the current time has to
// fall inside the ticket's validity window.
func checkInAllowed(ticket *model.Ticket, activity *model.Activity, now time.Time) error {
	if model.TicketStatus(ticket.Status) != model.TicketUnused {
		return ErrCheckInConflict
	}
	if model.ActivityStatus(activity.Status) == model.ActivityCancelled {
		return ErrCheckInConflict
	}
	if ticket.ValidStartAt != nil && now.Before(*ticket.ValidStartAt) {
		return ErrCheckInConflict
	}
	if ticket.ValidEndAt != nil && now.After(*ticket.ValidEndAt) {
		return ErrCheckInConflict
	}
	return nil
}
