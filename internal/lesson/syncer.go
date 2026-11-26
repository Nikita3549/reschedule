package lesson

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"reschedule/internal/week"
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
	ListBetween(ctx context.Context, from, to time.Time) ([]Lesson, error)
}

type WeekRepo interface {
	IDsByDates(ctx context.Context, dates []time.Time) ([]string, error)
	Create(ctx context.Context, startAt time.Time) (string, error)
	ByID(ctx context.Context, id string) (*week.Week, error)
}

type Notifier interface {
	Notify(ctx context.Context, ch Changes) error
	PublishWeek(ctx context.Context, weekID string, ls []Lesson) error
	EditWeekMessages(ctx context.Context, weekID string, ls []Lesson) error
}

type Syncer struct {
	lessons  Storer
	weekRepo WeekRepo
	fetcher  Fetcher
	notifier Notifier
}

func NewSyncer(repo Storer, weekRepo WeekRepo, f Fetcher, n Notifier) *Syncer {
	return &Syncer{
		lessons:  repo,
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

	bootstrap, err := s.lessons.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check bootstrap: %w", err)
	}

	stored, err := s.lessons.ListFrom(ctx, now)
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

	if err := s.lessons.Apply(ctx, changes); err != nil {
		return fmt.Errorf("apply changes: %w", err)
	}

	if bootstrap || visible.HasNoChangeAlerts() {
		return nil
	}

	allChanges := append(changes.Added, changes.Canceled...)
	for _, change := range changes.Changed {
		allChanges = append(allChanges, change.Old)
	}

	uniqueMondays := getUniqueMondays(allChanges)

	weekIDs, err := s.weekRepo.IDsByDates(ctx, uniqueMondays)
	if err != nil {
		return fmt.Errorf("get week ids by dates %d: %w", len(uniqueMondays), err)
	}

	for _, weekID := range weekIDs {
		week, err := s.weekRepo.ByID(ctx, weekID)
		if err != nil {
			log.Printf("get week by id: %v", err)
			continue
		}
		ls, err := s.lessons.ListBetween(ctx, week.StartAt, week.End())
		if err != nil {
			log.Printf("list weeks between: %v", err)
			continue
		}

		err = s.notifier.EditWeekMessages(ctx, weekID, ls)
		if err != nil {
			log.Printf("edit week messages: %v", err)
		}
	}

	if err := s.notifier.Notify(ctx, visible); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}

func getUniqueMondays(lessons []Lesson) []time.Time {
	mondays := make(map[time.Time]struct{})

	for _, lesson := range lessons {
		mondays[mondayOf(lesson.StartAt)] = struct{}{}
	}

	keys := make([]time.Time, 0, len(mondays))
	for k := range mondays {
		keys = append(keys, k)
	}

	return keys
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
