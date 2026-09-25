package service

import (
	"context"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
)

type CheckInView struct {
	ID          string           `json:"id"`
	TicketID    string           `json:"ticket_id"`
	ActivityID  string           `json:"activity_id"`
	User        CheckInUserView  `json:"user"`
	OperatorID  string           `json:"operator_id"`
	CheckedInAt timestamp.Millis `json:"checked_in_at"`
}

type CheckInUserView struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

func (s *CheckInService) buildCheckInViews(ctx context.Context, records []model.CheckIn) ([]CheckInView, error) {
	views := make([]CheckInView, 0, len(records))
	if len(records) == 0 {
		return views, nil
	}
	ticketIDs := make([]uint64, 0, len(records))
	activityIDs := make([]uint64, 0, len(records))
	userIDs := make([]uint64, 0, len(records)*2)
	for _, record := range records {
		ticketIDs = append(ticketIDs, record.TicketID)
		activityIDs = append(activityIDs, record.ActivityID)
		userIDs = append(userIDs, record.UserID, record.OperatorID)
	}
	tickets, err := s.tickets.FindByIDs(ctx, uniqueUint64(ticketIDs))
	if err != nil {
		return nil, err
	}
	ticketUUID := make(map[uint64]string, len(tickets))
	for _, ticket := range tickets {
		ticketUUID[ticket.ID] = ticket.UUID
	}
	activities, err := s.activities.FindByIDs(ctx, uniqueUint64(activityIDs))
	if err != nil {
		return nil, err
	}
	activityUUID := make(map[uint64]string, len(activities))
	for _, activity := range activities {
		activityUUID[activity.ID] = activity.UUID
	}
	users, err := s.users.FindByIDs(ctx, uniqueUint64(userIDs))
	if err != nil {
		return nil, err
	}
	userByID := make(map[uint64]model.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}
	for _, record := range records {
		view := CheckInView{
			ID:          record.UUID,
			TicketID:    ticketUUID[record.TicketID],
			ActivityID:  activityUUID[record.ActivityID],
			CheckedInAt: timestamp.Millis(record.CheckedInAt),
		}
		if user, ok := userByID[record.UserID]; ok {
			view.User = CheckInUserView{ID: user.UUID, Nickname: user.Nickname}
		}
		if operator, ok := userByID[record.OperatorID]; ok {
			view.OperatorID = operator.UUID
		}
		views = append(views, view)
	}
	return views, nil
}
