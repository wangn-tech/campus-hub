package repository

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

// Ticket codes avoid characters that are easy to confuse when read aloud.
const (
	ticketCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	ticketCodeLength   = 10
	ticketCodeAttempts = 5
)

type TicketRepository struct{ db *gorm.DB }

func NewTicketRepository(db *gorm.DB) *TicketRepository { return &TicketRepository{db: db} }

func (r *TicketRepository) FindByUUID(ctx context.Context, uuid string) (*model.Ticket, error) {
	var ticket model.Ticket
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketRepository) FindByRegistrationIDs(ctx context.Context, ids []uint64) ([]model.Ticket, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var tickets []model.Ticket
	err := r.db.WithContext(ctx).Where("registration_id IN ?", ids).Find(&tickets).Error
	return tickets, err
}

func (r *TicketRepository) FindByRegistrationID(ctx context.Context, id uint64) (*model.Ticket, error) {
	var ticket model.Ticket
	if err := r.db.WithContext(ctx).Where("registration_id = ?", id).First(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketRepository) ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]model.Ticket, int64, error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.Ticket{}).Where("user_id = ?", userID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tickets []model.Ticket
	err := base().Select("tickets.*").
		Order("tickets.issued_at DESC, tickets.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tickets).Error
	if err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

// NextCode returns an unused ticket code. The unique index on tickets.code is
// the authority, so the existence check only keeps collisions out of the write
// path.
func (r *TicketRepository) NextCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < ticketCodeAttempts; attempt++ {
		code, err := randomTicketCode()
		if err != nil {
			return "", err
		}
		var count int64
		if err := r.db.WithContext(ctx).Model(&model.Ticket{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("could not generate a unique ticket code")
}

func randomTicketCode() (string, error) {
	buf := make([]byte, ticketCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, ticketCodeLength)
	for i, b := range buf {
		code[i] = ticketCodeAlphabet[int(b)%len(ticketCodeAlphabet)]
	}
	return string(code), nil
}
