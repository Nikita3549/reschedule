package lessons

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) IsEmpty(ctx context.Context) (bool, error) {
	var ids []string

	err := r.db.WithContext(ctx).Model(&Lesson{}).Limit(1).Pluck("id", &ids).Error
	if err != nil {
		return false, fmt.Errorf("check lessons empty: %w", err)
	}

	return len(ids) == 0, nil
}

func (r *Repo) ListFrom(ctx context.Context, from time.Time) ([]Lesson, error) {
	var lessons []Lesson

	err := r.db.WithContext(ctx).
		Where("start_at >= ?", from).
		Find(&lessons).Error
	if err != nil {
		return nil, fmt.Errorf("list lessons from %s: %w", from, err)
	}

	return lessons, nil
}

func (r *Repo) Apply(ctx context.Context, ch Changes) error {
	updated := make([]Lesson, 0, len(ch.Changed))
	for _, c := range ch.Changed {
		updated = append(updated, c.New)
	}

	canceledIDs := make([]string, 0, len(ch.Canceled))
	for _, l := range ch.Canceled {
		canceledIDs = append(canceledIDs, l.ID)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createMany(tx, ch.Added); err != nil {
			return err
		}
		if err := updateMany(tx, updated); err != nil {
			return err
		}
		return deleteMany(tx, canceledIDs)
	})
}

func createMany(tx *gorm.DB, lessons []Lesson) error {
	if len(lessons) == 0 {
		return nil
	}
	err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lessons).Error
	if err != nil {
		return fmt.Errorf("create %d lessons: %w", len(lessons), err)
	}
	return nil
}

func updateMany(tx *gorm.DB, lessons []Lesson) error {
	for _, l := range lessons {
		err := tx.Model(&Lesson{}).Where("id = ?", l.ID).Select("*").Updates(l).Error
		if err != nil {
			return fmt.Errorf("update lesson %s: %w", l.ID, err)
		}
	}
	return nil
}

func deleteMany(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	err := tx.Where("id IN ?", ids).Delete(&Lesson{}).Error
	if err != nil {
		return fmt.Errorf("delete many lessons %v: %w", ids, err)
	}
	return nil
}
