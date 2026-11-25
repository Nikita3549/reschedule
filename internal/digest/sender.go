package digest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"reschedule/internal/lesson"
	"reschedule/internal/week"
)

const (
	DefaultHour = 20
)

type Lister interface {
	ListBetween(ctx context.Context, from, to time.Time) ([]lesson.Lesson, error)
}

type WeekRepo interface {
	ByDate(ctx context.Context, date time.Time) (*week.Week, error)
}

type Notifier interface {
	NotifyDaily(ctx context.Context, ls []lesson.Lesson) error
	NotifyNoLessonsNextWeek(ctx context.Context) error
	RotateWeekPins(ctx context.Context, weekID string) error
}

type Sender struct {
	lessons  Lister
	notifier Notifier
	weeks    WeekRepo
	hour     int
	now      func() time.Time
}

func New(lister Lister, notifier Notifier, weekRepo WeekRepo) *Sender {
	return &Sender{
		lessons:  lister,
		notifier: notifier,
		weeks:    weekRepo,
		hour:     DefaultHour,
		now:      time.Now,
	}
}

func (s *Sender) Run(ctx context.Context) {
	t := time.NewTimer(s.untilNextRun())
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if s.now().In(lesson.TZ).Weekday() == time.Saturday {
				err := s.pinNextWeek(ctx)
				if err != nil {
					log.Printf("pin next week: %v", err)
				}
			}
			if err := s.send(ctx); err != nil {
				log.Printf("digest send: %v", err)
			}
		case <-ctx.Done():
			return
		}

		t.Reset(s.untilNextRun())
	}
}

func (s *Sender) untilNextRun() time.Duration {
	now := s.now()
	return nextRun(now, s.hour).Sub(now)
}

func (s *Sender) pinNextWeek(ctx context.Context) error {
	monday := tomorrow(s.now()).AddDate(0, 0, 1)
	next, err := s.weeks.ByDate(ctx, monday)
	if errors.Is(err, week.ErrNotFound) {
		if err := s.notifier.NotifyNoLessonsNextWeek(ctx); err != nil {
			return fmt.Errorf("notify about empty next week: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get next week: %w", err)
	}

	return s.notifier.RotateWeekPins(ctx, next.ID)
}

func (s *Sender) send(ctx context.Context) error {
	from := tomorrow(s.now())

	ls, err := s.lessons.ListBetween(ctx, from, from.AddDate(0, 0, 1))
	if err != nil {
		return fmt.Errorf("list lessons: %w", err)
	}
	if len(ls) == 0 {
		return nil
	}

	if err := s.notifier.NotifyDaily(ctx, ls); err != nil {
		return fmt.Errorf("notify daily: %w", err)
	}
	return nil
}

func tomorrow(now time.Time) time.Time {
	t := now.In(lesson.TZ)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, lesson.TZ)
}

func nextRun(now time.Time, hour int) time.Time {
	t := now.In(lesson.TZ)

	run := time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, lesson.TZ)
	if !run.After(now) {
		run = run.AddDate(0, 0, 1)
	}
	return run
}
