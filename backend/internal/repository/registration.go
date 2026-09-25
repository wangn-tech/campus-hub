package repository

import (
	"context"
	"errors"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"gorm.io/gorm"
)

// ErrCapacityFull is returned when the guarded participant counter update finds
// no free slot, meaning the activity is full.
var ErrCapacityFull = errors.New("activity capacity full")

type RegistrationRepository struct{ db *gorm.DB }

func NewRegistrationRepository(db *gorm.DB) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

// RegisterInput is the atomic unit of work for a new registration: the slot is
// reserved, the registration row, its optional ticket, the status log and the
// outbox events are written in one transaction.
type RegisterInput struct {
	Registration *model.Registration
	Ticket       *model.Ticket // nil while the registration waits for approval
	Log          *model.RegistrationStatusLog
	Events       []*model.OutboxEvent
	Approved     bool // reserve the approved counter instead of the pending one
}

func (r *RegistrationRepository) Register(ctx context.Context, in RegisterInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pendingDelta, approvedDelta := 0, 0
		if in.Approved {
			approvedDelta = 1
		} else {
			pendingDelta = 1
		}
		if err := adjustParticipantCounters(tx, in.Registration.ActivityID, pendingDelta, approvedDelta, true); err != nil {
			return err
		}
		if err := tx.Create(in.Registration).Error; err != nil {
			return err
		}
		if in.Ticket != nil {
			in.Ticket.RegistrationID = in.Registration.ID
			if err := tx.Create(in.Ticket).Error; err != nil {
				return err
			}
		}
		if in.Log != nil {
			in.Log.RegistrationID = in.Registration.ID
			if err := tx.Create(in.Log).Error; err != nil {
				return err
			}
		}
		return enqueueOutboxEvents(tx, in.Events)
	})
}

// TransitionInput is the atomic unit of work for a registration status change,
// including the participant counter adjustments, the optional ticket change,
// the status log and the outbox events.
type TransitionInput struct {
	RegistrationID uint64
	ActivityID     uint64
	FromStatus     uint8
	Values         map[string]any
	PendingDelta   int
	ApprovedDelta  int
	// GuardCapacity only applies when the transition reserves a new slot; an
	// approval moves an already reserved slot and must not re-check capacity.
	GuardCapacity bool
	Ticket        *model.Ticket // issued when the transition approves the registration
	TicketID      *uint64       // voided when the transition releases a ticket
	Log           *model.RegistrationStatusLog
	Events        []*model.OutboxEvent
}

func (r *RegistrationRepository) Transition(ctx context.Context, in TransitionInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Registration{}).
			Where("id = ? AND status = ?", in.RegistrationID, in.FromStatus).
			Updates(in.Values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConcurrentUpdate
		}
		if err := adjustParticipantCounters(tx, in.ActivityID, in.PendingDelta, in.ApprovedDelta, in.GuardCapacity); err != nil {
			return err
		}
		if in.Ticket != nil {
			in.Ticket.RegistrationID = in.RegistrationID
			if err := tx.Create(in.Ticket).Error; err != nil {
				return err
			}
		}
		if in.TicketID != nil {
			now := time.Now().UTC()
			err := tx.Model(&model.Ticket{}).
				Where("id = ? AND status = ?", *in.TicketID, uint8(model.TicketUnused)).
				Updates(map[string]any{
					"status":     uint8(model.TicketVoided),
					"voided_at":  now,
					"updated_at": now,
					"version":    gorm.Expr("version + 1"),
				}).Error
			if err != nil {
				return err
			}
		}
		if err := tx.Create(in.Log).Error; err != nil {
			return err
		}
		return enqueueOutboxEvents(tx, in.Events)
	})
}

func (r *RegistrationRepository) FindByUUID(ctx context.Context, uuid string) (*model.Registration, error) {
	var registration model.Registration
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&registration).Error; err != nil {
		return nil, err
	}
	return &registration, nil
}

// FindActive returns the single active registration of a user for an activity,
// if any. Active means pending or approved (see the uk_active_registration key).
func (r *RegistrationRepository) FindActive(ctx context.Context, activityID, userID uint64) (*model.Registration, error) {
	var registration model.Registration
	err := r.db.WithContext(ctx).
		Where("activity_id = ? AND user_id = ? AND active_flag = 1", activityID, userID).
		First(&registration).Error
	if err != nil {
		return nil, err
	}
	return &registration, nil
}

func (r *RegistrationRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Registration, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var registrations []model.Registration
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&registrations).Error
	return registrations, err
}

// UserRegistrationFilter describes the "my registrations" query.
type UserRegistrationFilter struct {
	UserID   uint64
	Statuses []int
	Upcoming *bool // true: the activity has not started yet, false: it has
	Page     int
	PageSize int
}

func (r *RegistrationRepository) ListByUser(ctx context.Context, filter UserRegistrationFilter) ([]model.Registration, int64, error) {
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&model.Registration{}).
			Joins("JOIN activities ON activities.id = activity_registrations.activity_id").
			Where("activity_registrations.user_id = ? AND activities.deleted_at IS NULL", filter.UserID)
		if len(filter.Statuses) > 0 {
			query = query.Where("activity_registrations.status IN ?", filter.Statuses)
		}
		if filter.Upcoming != nil {
			if *filter.Upcoming {
				query = query.Where("activities.activity_start_at >= ?", time.Now().UTC())
			} else {
				query = query.Where("activities.activity_start_at < ?", time.Now().UTC())
			}
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var registrations []model.Registration
	err := base().Select("activity_registrations.*").
		Order("activity_registrations.created_at DESC, activity_registrations.id DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&registrations).Error
	if err != nil {
		return nil, 0, err
	}
	return registrations, total, nil
}

// ListByActivity returns the registrations of one activity for its organizer.
func (r *RegistrationRepository) ListByActivity(ctx context.Context, activityID uint64, statuses []int, page, pageSize int) ([]model.Registration, int64, error) {
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&model.Registration{}).Where("activity_id = ?", activityID)
		if len(statuses) > 0 {
			query = query.Where("status IN ?", statuses)
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var registrations []model.Registration
	err := base().
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&registrations).Error
	if err != nil {
		return nil, 0, err
	}
	return registrations, total, nil
}

// ListExpiredPending returns the pending registrations whose approval window has
// passed, oldest first.
func (r *RegistrationRepository) ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]model.Registration, error) {
	var registrations []model.Registration
	err := r.db.WithContext(ctx).
		Where("status = ? AND active_flag = 1 AND expires_at IS NOT NULL AND expires_at <= ?", uint8(model.RegistrationPending), now).
		Order("expires_at").
		Limit(limit).
		Find(&registrations).Error
	return registrations, err
}

// adjustParticipantCounters moves the pending/approved participant counters of
// an activity. When guard is set the update only applies while there is still a
// free slot, which is how overbooking is prevented.
func adjustParticipantCounters(tx *gorm.DB, activityID uint64, pendingDelta, approvedDelta int, guard bool) error {
	if pendingDelta == 0 && approvedDelta == 0 {
		return nil
	}
	query := tx.Model(&model.Activity{}).Where("id = ?", activityID)
	if guard {
		query = query.Where("approved_participant_count + pending_participant_count < max_participants")
	}
	values := map[string]any{}
	if pendingDelta != 0 {
		values["pending_participant_count"] = gorm.Expr("GREATEST(CAST(pending_participant_count AS SIGNED) + ?, 0)", pendingDelta)
	}
	if approvedDelta != 0 {
		values["approved_participant_count"] = gorm.Expr("GREATEST(CAST(approved_participant_count AS SIGNED) + ?, 0)", approvedDelta)
	}
	result := query.Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if guard && result.RowsAffected == 0 {
		return ErrCapacityFull
	}
	return nil
}
