package users

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Save(ctx context.Context, u User) error {
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"chat_id", "username", "first_name", "last_name", "language_code", "updated_at",
			}),
		}).
		Create(&u).Error
	if err != nil {
		return fmt.Errorf("save user %d: %w", u.ID, err)
	}
	return nil
}

func (r *Repo) List(ctx context.Context) ([]User, error) {
	var users []User

	if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}
