package lesson

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"
)

const maxNewWeeks = 10

var ErrEmptySchedule = errors.New("empty fetched lessons slice")

var hiddenSubjects = map[string]struct{}{
	"Доп. виды обучения \"Русский язык как иностранный\"": {},
	"Информационный час":                  {},
	"Информационный час, Кураторский час": {},
}

type endShiftKey struct {
	Kind    Kind
	Subject string
}

var endShifts = map[endShiftKey]time.Duration{
	{Kind: KindLab, Subject: "Алгоритмизация и программирование"}:      5 * time.Minute,
	{Kind: KindPractice, Subject: "Алгоритмизация и программирование"}: 5 * time.Minute,
	{Kind: KindPractice, Subject: "Физическая культура"}:               -20 * time.Minute,
	{Kind: KindLecture, Subject: "Компьютерная графика"}:               5 * time.Minute,
	{Kind: KindLab, Subject: "Компьютерная графика"}:                   5 * time.Minute,
	{Kind: KindPractice, Subject: "Компьютерная графика"}:              5 * time.Minute,
	{Kind: KindLecture, Subject: "Высшая математика"}:                  5 * time.Minute,
	{Kind: KindLab, Subject: "Прикладная информатика"}:                 5 * time.Minute,
	{Kind: KindPractice, Subject: "Прикладная информатика"}:            5 * time.Minute,
}

type Fetcher interface {
	Fetch(ctx context.Context) ([]Lesson, error)
}

type Storer interface {
	IsEmpty(ctx context.Context) (bool, error)
	ListFrom(ctx context.Context, from time.Time) ([]Lesson, error)
	Apply(ctx context.Context, ch Changes) error
}

type WeekRepo interface {
	IDsByDates(ctx context.Context, dates []time.Time) ([]string, error)
	Create(ctx context.Context, startAt time.Time) (string, error)
}

type Notifier interface {
	Notify(ctx context.Context, ch Changes) error
	PublishWeek(ctx context.Context, weekID string, ls []Lesson) error
}

type Syncer struct {
	repo     Storer
	weekRepo WeekRepo
	fetcher  Fetcher
	notifier Notifier
}

func NewSyncer(repo Storer, weekRepo WeekRepo, f Fetcher, n Notifier) *Syncer {
	return &Syncer{
		repo:     repo,
		weekRepo: weekRepo,
		fetcher:  f,
		notifier: n,
	}
}

func (s *Syncer) Sync(ctx context.Context) error {
	fetched, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	now := time.Now()
	var tmp []Lesson
	for _, l := range fetched {
		if l.StartAt.After(now) {
			tmp = append(tmp, l)
		}
	}
	fetched = tmp

	if len(fetched) == 0 {
		return ErrEmptySchedule
	}

	markHidden(fetched)
	addEndShift(fetched)

	bootstrap, err := s.repo.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check bootstrap: %w", err)
	}

	stored, err := s.repo.ListFrom(ctx, now)
	if err != nil {
		return fmt.Errorf("load stored: %w", err)
	}
	fetchedDates := make([]time.Time, 0, len(fetched))
	for _, l := range fetched {
		fetchedDates = append(fetchedDates, l.StartAt)
	}

	ids, err := s.weekRepo.IDsByDates(ctx, fetchedDates)
	if err != nil {
		return fmt.Errorf("get week ids by dates: %w", err)
	}

	lessonIDsWithWeek := make(map[string]struct{})
	for i, l := range fetched {
		if ids[i] != "" {
			lessonIDsWithWeek[l.ID] = struct{}{}
		}
	}

	changes := Diff(stored, fetched, lessonIDsWithWeek)

	visible := changes.Visible()

	newLessons := slices.Clone(visible.NewWeekLessons)
	slices.SortFunc(newLessons, func(a, b Lesson) int { return a.StartAt.Compare(b.StartAt) })

	for i := 0; i < maxNewWeeks && len(newLessons) > 0; i++ {
		monday := mondayOf(newLessons[0].StartAt)

		weekID, err := s.weekRepo.Create(ctx, monday)
		if err != nil {
			return fmt.Errorf("create week: %w", err)
		}

		nextIdx := nextWeekFirstIdx(monday, newLessons)
		if nextIdx == -1 {
			nextIdx = len(newLessons)
		}
		if !bootstrap {
			err := s.notifier.PublishWeek(ctx, weekID, newLessons[:nextIdx])
			if err != nil {
				return fmt.Errorf("publish week: %w", err)
			}
		}

		if i == maxNewWeeks-1 {
			log.Printf("publish new weeks: got more than %d new weeks from fetcher: %d", maxNewWeeks, len(visible.NewWeekLessons))
		}

		newLessons = newLessons[nextIdx:]
	}

	if err := s.repo.Apply(ctx, changes); err != nil {
		return fmt.Errorf("apply changes: %w", err)
	}

	if bootstrap || visible.HasChangeAlerts() {
		return nil
	}

	if err := s.notifier.Notify(ctx, visible); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}

func addEndShift(lessons []Lesson) {
	for i := range lessons {
		if shift, ok := endShifts[endShiftKey{Kind: lessons[i].Kind, Subject: lessons[i].Subject}]; ok {
			lessons[i].EndAt = lessons[i].EndAt.Add(shift)
		}
	}
}

func markHidden(lessons []Lesson) {
	for i := range lessons {
		if _, ok := hiddenSubjects[lessons[i].Subject]; ok {
			lessons[i].Hide = true
		}
	}
}

func mondayOf(startAt time.Time) time.Time {
	t := startAt.In(TZ)

	shift := int(t.Weekday()) - 1
	if shift == -1 {
		shift = 6
	}

	return time.Date(t.Year(), t.Month(), t.Day()-shift, 0, 0, 0, 0, TZ)
}

func nextWeekFirstIdx(startDay time.Time, lessons []Lesson) int {
	for idx, l := range lessons {
		if !l.StartAt.Before(startDay.AddDate(0, 0, 7)) {
			return idx
		}
	}
	return -1
}
