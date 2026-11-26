package weekmsg

import (
	"context"

	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{
		db: db,
	}
}

func (r *Repo) CreateMany(ctx context.Context, wm []Message) error {
	if len(wm) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&wm).Error
}

func (r *Repo) ListByWeekID(ctx context.Context, weekID string) ([]Message, error) {
	var msgs []Message
	err := r.db.WithContext(ctx).Where("week_id = ?", weekID).Find(&msgs).Error
	return msgs, err
}

func (r *Repo) ListForPinRotation(ctx context.Context, weekID string) ([]Message, error) {
	var msgs []Message
	err := r.db.WithContext(ctx).
		Where("week_id = ? OR pin_state <> ?", weekID, Unpinned).
		Find(&msgs).Error
	return msgs, err
}

func (r *Repo) SetPinState(ctx context.Context, msg Message, state PinState) error {
	res := r.db.WithContext(ctx).Model(&msg).Update("pin_state", state)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
