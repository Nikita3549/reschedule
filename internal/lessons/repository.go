package lessons

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

func (r *Repo) FilterNew(ctx context.Context, lessons []Lesson) ([]Lesson, error) {
	if len(lessons) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(lessons))
	for _, l := range lessons {
		ids = append(ids, l.ID)
	}

	var existing []string
	err := r.db.WithContext(ctx).
		Model(&Lesson{}).
		Where("id IN ?", ids).
		Pluck("id", &existing).Error
	if err != nil {
		return nil, fmt.Errorf("select existing lesson ids: %w", err)
	}

	exists := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		exists[id] = struct{}{}
	}

	var fresh []Lesson
	for _, l := range lessons {
		if _, ok := exists[l.ID]; !ok {
			fresh = append(fresh, l)
		}
	}

	return fresh, nil
}

func (r *Repo) CreateMany(ctx context.Context, lessons []Lesson) error {
	if len(lessons) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&lessons).Error
}
