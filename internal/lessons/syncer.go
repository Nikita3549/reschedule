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
	IsEmpty(ctx context.Context) (bool, error)
	ListFrom(ctx context.Context, from time.Time) ([]Lesson, error)
	Apply(ctx context.Context, ch Changes) error
}

type Notifier interface {
	Notify(ctx context.Context, ch Changes) error
}

type Syncer struct {
	repo     Storer
	fetcher  Fetcher
	notifier Notifier
}

func NewSyncer(repo Storer, f Fetcher, n Notifier) *Syncer {
	return &Syncer{
		repo:     repo,
		fetcher:  f,
		notifier: n,
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

	bootstrap, err := s.repo.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check bootstrap: %w", err)
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

	if bootstrap || changes.Empty() {
		return nil
	}
	if err := s.notifier.Notify(ctx, changes); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}
