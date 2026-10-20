package lessons

import (
	"context"
	"fmt"
)

type Fetcher interface {
	Fetch(ctx context.Context) ([]Lesson, error)
}

type Storer interface {
	FilterNew(ctx context.Context, lessons []Lesson) ([]Lesson, error)
	CreateMany(ctx context.Context, lessons []Lesson) error
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

func (s *Syncer) Sync(ctx context.Context) (err error) {
	lessons, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	newLessons, err := s.repo.FilterNew(ctx, lessons)
	if err != nil {
		return fmt.Errorf("filter: %w", err)
	}

	err = s.repo.CreateMany(ctx, newLessons)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}
