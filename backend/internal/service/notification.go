package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
	"github.com/wangn-tech/campus-hub/internal/repository"
)

// ErrNotificationInvalid is returned when the request body is malformed.
var ErrNotificationInvalid = errors.New("notification invalid input")

type NotificationService struct {
	notifications *repository.NotificationRepository
}

func NewNotificationService(notifications *repository.NotificationRepository) *NotificationService {
	return &NotificationService{notifications: notifications}
}

type NotificationView struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	Data      json.RawMessage  `json:"data,omitempty"`
	IsRead    bool             `json:"is_read"`
	ReadAt    timestamp.Millis `json:"read_at"`
	CreatedAt timestamp.Millis `json:"created_at"`
}

func (s *NotificationService) List(ctx context.Context, user *model.User, page, pageSize int) ([]NotificationView, int64, error) {
	page, pageSize = NormalizePage(page, pageSize)
	notifications, total, err := s.notifications.ListByUser(ctx, user.ID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	views := make([]NotificationView, 0, len(notifications))
	for _, notification := range notifications {
		view := NotificationView{
			ID:        notification.UUID,
			Type:      notification.Type,
			Title:     notification.Title,
			Content:   notification.Content,
			IsRead:    notification.IsRead,
			ReadAt:    millisOrZero(notification.ReadAt),
			CreatedAt: timestamp.Millis(notification.CreatedAt),
		}
		if notification.Data != nil && *notification.Data != "" {
			view.Data = json.RawMessage(*notification.Data)
		}
		views = append(views, view)
	}
	return views, total, nil
}

func (s *NotificationService) UnreadCount(ctx context.Context, user *model.User) (int64, error) {
	return s.notifications.UnreadCount(ctx, user.ID)
}

// MarkRead marks the caller's notifications read. It is idempotent: marking an
// already read notification again reports zero changed rows.
func (s *NotificationService) MarkRead(ctx context.Context, user *model.User, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, ErrNotificationInvalid
	}
	return s.notifications.MarkRead(ctx, user.ID, uniqueStrings(ids))
}

func (s *NotificationService) MarkAllRead(ctx context.Context, user *model.User) (int64, error) {
	return s.notifications.MarkAllRead(ctx, user.ID)
}
