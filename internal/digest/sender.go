package digest

import (
	"context"
	"fmt"
	"log"
	"time"

	"reschedule/internal/lessons"
)

const DefaultHour = 20

type Lister interface {
	ListBetween(ctx context.Context, from, to time.Time) ([]lessons.Lesson, error)
}

type Notifier interface {
	NotifyDaily(ctx context.Context, ls []lessons.Lesson) error
}

type Sender struct {
	lessons  Lister
	notifier Notifier
	hour     int
	now      func() time.Time
}

func New(lister Lister, notifier Notifier) *Sender {
	return &Sender{
		lessons:  lister,
		notifier: notifier,
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
	t := now.In(lessons.TZ)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, lessons.TZ)
}

func nextRun(now time.Time, hour int) time.Time {
	t := now.In(lessons.TZ)

	run := time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, lessons.TZ)
	if !run.After(now) {
		run = run.AddDate(0, 0, 1)
	}
	return run
}
