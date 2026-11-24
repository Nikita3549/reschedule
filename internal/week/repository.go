package week

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("week not found")

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{
		db: db,
	}
}

func (r *Repo) Create(ctx context.Context, startAt time.Time) (string, error) {
	w := Week{StartAt: startAt}
	err := r.db.WithContext(ctx).Save(&w).Error
	if err != nil {
		return "", fmt.Errorf("create week: %w", err)
	}
	return w.ID, nil
}

func (r *Repo) ByDate(ctx context.Context, date time.Time) (*Week, error) {
	var week Week
	err := r.db.WithContext(ctx).
		Where("start_at <= ? AND start_at > ?", date, date.AddDate(0, 0, -7)).
		Order("start_at DESC").
		First(&week).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get week by date %s: %w", date.Format("2006-01-02 15:04:05"), err)
	}

	return &week, nil
}

func (r *Repo) IDsByDates(ctx context.Context, dates []time.Time) ([]string, error) {
	if len(dates) == 0 {
		return []string{}, nil
	}

	var rows []struct {
		Idx    int
		WeekID string
	}

	err := r.db.Table(Week{}.TableName()).
		WithContext(ctx).
		Select("dates_tbl.idx, weeks.id AS week_id").
		Joins(`JOIN unnest(?::TIMESTAMPTZ[]) WITH ORDINALITY AS dates_tbl(val, idx) ON
			weeks.start_at <= dates_tbl.val AND dates_tbl.val - INTERVAL '1 week' < weeks.start_at`, pq.Array(dates)).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get week ids by %d dates: %w", len(dates), err)
	}

	ids := make([]string, len(dates))
	for _, row := range rows {
		ids[row.Idx-1] = row.WeekID
	}

	return ids, nil
}

func (r *Repo) Next(ctx context.Context, w *Week) (*Week, error) {
	return r.ByDate(ctx, w.StartAt.AddDate(0, 0, 7))
}
