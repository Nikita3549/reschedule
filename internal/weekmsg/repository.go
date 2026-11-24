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
