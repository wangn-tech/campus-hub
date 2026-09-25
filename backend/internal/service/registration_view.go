package service

import (
	"context"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
)

type RegistrationView struct {
	ID           string           `json:"id"`
	Status       uint8            `json:"status"`
	RejectReason string           `json:"reject_reason"`
	ExpiresAt    timestamp.Millis `json:"expires_at"`
	DecidedAt    timestamp.Millis `json:"decided_at"`
	CancelTime   timestamp.Millis `json:"cancel_time"`
	CreatedAt    timestamp.Millis `json:"created_at"`
	UpdatedAt    timestamp.Millis `json:"updated_at"`
	Activity     ActivityBrief    `json:"activity"`
	User         *UserBrief       `json:"user,omitempty"`
	Ticket       *TicketView      `json:"ticket,omitempty"`
}

type ActivityBrief struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	CoverURL        string           `json:"cover_url"`
	Location        string           `json:"location"`
	ActivityStartAt timestamp.Millis `json:"activity_start_at"`
	ActivityEndAt   timestamp.Millis `json:"activity_end_at"`
	Status          uint8            `json:"status"`
}

type UserBrief struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type TicketView struct {
	ID             string           `json:"id"`
	Code           string           `json:"code"`
	Status         uint8            `json:"status"`
	RegistrationID string           `json:"registration_id"`
	Activity       ActivityBrief    `json:"activity"`
	ValidStartAt   timestamp.Millis `json:"valid_start_at"`
	ValidEndAt     timestamp.Millis `json:"valid_end_at"`
	IssuedAt       timestamp.Millis `json:"issued_at"`
	UsedAt         timestamp.Millis `json:"used_at"`
	VoidedAt       timestamp.Millis `json:"voided_at"`
}

func (s *RegistrationService) buildRegistrationViews(ctx context.Context, registrations []model.Registration, withUser bool) ([]RegistrationView, error) {
	views := make([]RegistrationView, 0, len(registrations))
	if len(registrations) == 0 {
		return views, nil
	}
	activityIDs := make([]uint64, 0, len(registrations))
	registrationIDs := make([]uint64, 0, len(registrations))
	userIDs := make([]uint64, 0, len(registrations))
	for _, registration := range registrations {
		activityIDs = append(activityIDs, registration.ActivityID)
		registrationIDs = append(registrationIDs, registration.ID)
		userIDs = append(userIDs, registration.UserID)
	}
	activities, err := s.activities.FindByIDs(ctx, uniqueUint64(activityIDs))
	if err != nil {
		return nil, err
	}
	coverURLs, err := presignCoverURLs(ctx, s.files, s.attachments, activities)
	if err != nil {
		return nil, err
	}
	activityByID := make(map[uint64]model.Activity, len(activities))
	for _, activity := range activities {
		activityByID[activity.ID] = activity
	}
	tickets, err := s.tickets.FindByRegistrationIDs(ctx, uniqueUint64(registrationIDs))
	if err != nil {
		return nil, err
	}
	ticketByRegistration := make(map[uint64]TicketView, len(tickets))
	for _, ticket := range tickets {
		ticketByRegistration[ticket.RegistrationID] = ticketBrief(ticket, activityByID[ticket.ActivityID], coverURLs)
	}
	userByID := map[uint64]UserBrief{}
	if withUser {
		users, err := s.users.FindByIDs(ctx, uniqueUint64(userIDs))
		if err != nil {
			return nil, err
		}
		avatarURLs, err := presignAvatarURLs(ctx, s.files, s.attachments, users)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			userByID[user.ID] = UserBrief{ID: user.UUID, Nickname: user.Nickname, AvatarURL: avatarURLs[user.ID]}
		}
	}
	for _, registration := range registrations {
		view := RegistrationView{
			ID:           registration.UUID,
			Status:       registration.Status,
			RejectReason: registration.RejectReason,
			ExpiresAt:    millisOrZero(registration.ExpiresAt),
			DecidedAt:    millisOrZero(registration.DecidedAt),
			CancelTime:   millisOrZero(registration.CancelTime),
			CreatedAt:    timestamp.Millis(registration.CreatedAt),
			UpdatedAt:    timestamp.Millis(registration.UpdatedAt),
			Activity:     activityBrief(activityByID[registration.ActivityID], coverURLs),
		}
		if withUser {
			if user, ok := userByID[registration.UserID]; ok {
				brief := user
				view.User = &brief
			}
		}
		if ticket, ok := ticketByRegistration[registration.ID]; ok {
			brief := ticket
			view.Ticket = &brief
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *RegistrationService) buildTicketViews(ctx context.Context, tickets []model.Ticket) ([]TicketView, error) {
	views := make([]TicketView, 0, len(tickets))
	if len(tickets) == 0 {
		return views, nil
	}
	activityIDs := make([]uint64, 0, len(tickets))
	registrationIDs := make([]uint64, 0, len(tickets))
	for _, ticket := range tickets {
		activityIDs = append(activityIDs, ticket.ActivityID)
		registrationIDs = append(registrationIDs, ticket.RegistrationID)
	}
	activities, err := s.activities.FindByIDs(ctx, uniqueUint64(activityIDs))
	if err != nil {
		return nil, err
	}
	coverURLs, err := presignCoverURLs(ctx, s.files, s.attachments, activities)
	if err != nil {
		return nil, err
	}
	activityByID := make(map[uint64]model.Activity, len(activities))
	for _, activity := range activities {
		activityByID[activity.ID] = activity
	}
	registrations, err := s.registrations.FindByIDs(ctx, uniqueUint64(registrationIDs))
	if err != nil {
		return nil, err
	}
	registrationUUID := make(map[uint64]string, len(registrations))
	for _, registration := range registrations {
		registrationUUID[registration.ID] = registration.UUID
	}
	for _, ticket := range tickets {
		view := ticketBrief(ticket, activityByID[ticket.ActivityID], coverURLs)
		view.RegistrationID = registrationUUID[ticket.RegistrationID]
		views = append(views, view)
	}
	return views, nil
}

func ticketBrief(ticket model.Ticket, activity model.Activity, coverURLs map[uint64]string) TicketView {
	return TicketView{
		ID:           ticket.UUID,
		Code:         ticket.Code,
		Status:       ticket.Status,
		Activity:     activityBrief(activity, coverURLs),
		ValidStartAt: millisOrZero(ticket.ValidStartAt),
		ValidEndAt:   millisOrZero(ticket.ValidEndAt),
		IssuedAt:     timestamp.Millis(ticket.IssuedAt),
		UsedAt:       millisOrZero(ticket.UsedAt),
		VoidedAt:     millisOrZero(ticket.VoidedAt),
	}
}

func activityBrief(activity model.Activity, coverURLs map[uint64]string) ActivityBrief {
	brief := ActivityBrief{
		ID:              activity.UUID,
		Title:           activity.Title,
		Location:        activity.Location,
		ActivityStartAt: timestamp.Millis(activity.ActivityStartAt),
		ActivityEndAt:   timestamp.Millis(activity.ActivityEndAt),
		Status:          activity.Status,
	}
	if activity.CoverFileID != nil {
		brief.CoverURL = coverURLs[*activity.CoverFileID]
	}
	return brief
}

func millisOrZero(value *time.Time) timestamp.Millis {
	if value == nil {
		return timestamp.Millis{}
	}
	return timestamp.Millis(*value)
}
