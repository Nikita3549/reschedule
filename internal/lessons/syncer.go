package lessons

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrEmptySchedule = errors.New("empty fetched lessons slice")

type Fetcher interface {
	Fetch(ctx context.Context) ([]Lesson, error)
}

type Storer interface {
	ListFrom(ctx context.Context, from time.Time) ([]Lesson, error)
	Apply(ctx context.Context, ch Changes) error
}

type Syncer struct {
	repo    Storer
	fetcher Fetcher
}

func NewSyncer(repo Storer, f Fetcher) *Syncer {
	return &Syncer{
		repo:    repo,
		fetcher: f,
	}
}

func (s *Syncer) Sync(ctx context.Context) error {
	fetched, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	if len(fetched) == 0 {
		return ErrEmptySchedule
	}

	now := time.Now()
	stored, err := s.repo.ListFrom(ctx, now)
	if err != nil {
		return fmt.Errorf("load stored: %w", err)
	}

	var upcoming []Lesson
	for _, l := range fetched {
		if l.StartAt.After(now) {
			upcoming = append(upcoming, l)
		}
	}

	changes := Diff(stored, upcoming)

	if err := s.repo.Apply(ctx, changes); err != nil {
		return fmt.Errorf("apply changes: %w", err)
	}
	return nil
}
