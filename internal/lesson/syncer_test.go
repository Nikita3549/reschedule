package lesson

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

type fakeFetcher struct {
	lessons []Lesson
	err     error
}

func (f *fakeFetcher) Fetch(context.Context) ([]Lesson, error) {
	return f.lessons, f.err
}

type fakeStore struct {
	empty    bool
	emptyErr error
	stored   []Lesson
	listErr  error
	applyErr error

	applied Changes
	calls   []string
}

func (s *fakeStore) IsEmpty(context.Context) (bool, error) {
	s.calls = append(s.calls, "IsEmpty")
	return s.empty, s.emptyErr
}

func (s *fakeStore) ListFrom(context.Context, time.Time) ([]Lesson, error) {
	s.calls = append(s.calls, "ListFrom")
	return s.stored, s.listErr
}

func (s *fakeStore) Apply(_ context.Context, ch Changes) error {
	s.calls = append(s.calls, "Apply")
	s.applied = ch
	return s.applyErr
}

type fakeWeekRepo struct {
	noWeeksFrom time.Time
	err         error
	createErr   error

	created []time.Time
}

func (w *fakeWeekRepo) IDsByDates(_ context.Context, dates []time.Time) ([]string, error) {
	if w.err != nil {
		return nil, w.err
	}
	ids := make([]string, len(dates))
	for i, d := range dates {
		if w.noWeeksFrom.IsZero() || d.Before(w.noWeeksFrom) {
			ids[i] = "week"
		}
	}
	return ids, nil
}

func (w *fakeWeekRepo) Create(_ context.Context, startAt time.Time) (string, error) {
	if w.createErr != nil {
		return "", w.createErr
	}
	w.created = append(w.created, startAt)
	return "week", nil
}

type fakeNotifier struct {
	err       error
	notified  []Changes
	published []weekPublication
}

type weekPublication struct {
	weekID  string
	lessons []Lesson
}

func (n *fakeNotifier) PublishWeek(_ context.Context, weekID string, lessons []Lesson) error {
	n.published = append(n.published, weekPublication{weekID: weekID, lessons: slices.Clone(lessons)})
	return nil
}

func (n *fakeNotifier) Notify(_ context.Context, ch Changes) error {
	n.notified = append(n.notified, ch)
	return n.err
}

func futureLesson(id string) Lesson {
	start := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	l := testLesson(id)
	l.StartAt = start
	l.EndAt = start.Add(80 * time.Minute)
	return l
}

func TestSyncerSync(t *testing.T) {
	kept := futureLesson("kept")
	canceled := futureLesson("canceled")
	added := futureLesson("added")
	old := futureLesson("changed")
	changed := old
	changed.Room = "101"
	past := testLesson("past")

	stored := []Lesson{kept, canceled, old}
	fetched := []Lesson{kept, added, changed, past}
	errBoom := errors.New("boom")

	all := []string{"IsEmpty", "ListFrom", "Apply"}

	want := Changes{
		Added:    []Lesson{added},
		Canceled: []Lesson{canceled},
		Changed:  []Change{{Old: old, New: changed}},
	}

	tests := []struct {
		name         string
		fetcher      *fakeFetcher
		store        *fakeStore
		weeks        *fakeWeekRepo
		notifier     *fakeNotifier
		wantErr      error
		wantCalls    []string
		wantNotified []Changes
	}{
		{
			name:         "applies diff of future lessons and notifies",
			fetcher:      &fakeFetcher{lessons: fetched},
			store:        &fakeStore{stored: stored},
			wantCalls:    all,
			wantNotified: []Changes{want},
		},
		{
			name:      "bootstrap applies without notifying",
			fetcher:   &fakeFetcher{lessons: []Lesson{added}},
			store:     &fakeStore{empty: true},
			wantCalls: all,
		},
		{
			name:      "no changes does not notify",
			fetcher:   &fakeFetcher{lessons: []Lesson{kept}},
			store:     &fakeStore{stored: []Lesson{kept}},
			wantCalls: all,
		},
		{
			name:         "notify error is returned",
			fetcher:      &fakeFetcher{lessons: fetched},
			store:        &fakeStore{stored: stored},
			notifier:     &fakeNotifier{err: errBoom},
			wantErr:      errBoom,
			wantCalls:    all,
			wantNotified: []Changes{want},
		},
		{
			name:      "is empty error stops sync",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{emptyErr: errBoom},
			wantErr:   errBoom,
			wantCalls: all[:1],
		},
		{
			name:      "fetch error stops sync",
			fetcher:   &fakeFetcher{err: errBoom},
			store:     &fakeStore{},
			wantErr:   errBoom,
			wantCalls: nil,
		},
		{
			name:      "list error stops sync",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{listErr: errBoom},
			wantErr:   errBoom,
			wantCalls: all[:2],
		},
		{
			name:      "only past lessons is empty schedule",
			fetcher:   &fakeFetcher{lessons: []Lesson{past}},
			store:     &fakeStore{},
			wantErr:   ErrEmptySchedule,
			wantCalls: nil,
		},
		{
			name:      "weeks error stops sync",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{stored: stored},
			weeks:     &fakeWeekRepo{err: errBoom},
			wantErr:   errBoom,
			wantCalls: all[:2],
		},
		{
			name:      "apply error is returned",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{stored: stored, applyErr: errBoom},
			wantErr:   errBoom,
			wantCalls: all,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.notifier
			if n == nil {
				n = &fakeNotifier{}
			}
			w := tt.weeks
			if w == nil {
				w = &fakeWeekRepo{}
			}
			s := NewSyncer(tt.store, w, tt.fetcher, n)

			err := s.Sync(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sync() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.store.calls, tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", tt.store.calls, tt.wantCalls)
			}
			if !reflect.DeepEqual(n.notified, tt.wantNotified) {
				t.Fatalf("notified = %+v, want %+v", n.notified, tt.wantNotified)
			}
			if tt.wantErr != nil || len(tt.wantNotified) == 0 {
				return
			}
			if !reflect.DeepEqual(tt.store.applied, want) {
				t.Errorf("Apply got %+v, want %+v", tt.store.applied, want)
			}
		})
	}
}

func TestSyncerSyncHidden(t *testing.T) {
	kept := futureLesson("kept")
	hidden := futureLesson("hidden")
	hidden.Subject = "Информационный час"

	store := &fakeStore{stored: []Lesson{kept}}
	n := &fakeNotifier{}
	s := NewSyncer(store, &fakeWeekRepo{}, &fakeFetcher{lessons: []Lesson{kept, hidden}}, n)

	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	hidden.Hide = true
	if want := (Changes{Added: []Lesson{hidden}}); !reflect.DeepEqual(store.applied, want) {
		t.Errorf("Apply got %+v, want %+v", store.applied, want)
	}
	if len(n.notified) != 0 {
		t.Errorf("notified = %+v, want none", n.notified)
	}
}

func TestSyncerSyncWeeks(t *testing.T) {
	kept := futureLesson("kept")
	inWeek := futureLesson("in week")
	outOfWeek := futureLesson("out of week")
	start := outOfWeek.StartAt.In(TZ).AddDate(0, 0, 7)
	outOfWeek.StartAt = time.Date(start.Year(), start.Month(), start.Day(), 12, 0, 0, 0, TZ)
	outOfWeek.EndAt = outOfWeek.StartAt.Add(80 * time.Minute)
	secondLesson := outOfWeek
	secondLesson.ID = "second lesson"
	secondLesson.StartAt = secondLesson.StartAt.Add(time.Minute)
	secondLesson.EndAt = secondLesson.EndAt.Add(time.Minute)
	lastWeek := outOfWeek
	lastWeek.ID = "last week"
	lastWeek.StartAt = lastWeek.StartAt.AddDate(0, 0, 7)
	lastWeek.EndAt = lastWeek.EndAt.AddDate(0, 0, 7)
	errBoom := errors.New("boom")

	tests := []struct {
		name          string
		fetched       []Lesson
		createErr     error
		wantErr       error
		wantApplied   Changes
		wantNotified  []Changes
		wantCreated   []time.Time
		wantPublished []weekPublication
	}{
		{
			name:         "lesson with week is added and notified",
			fetched:      []Lesson{kept, inWeek},
			wantApplied:  Changes{Added: []Lesson{inWeek}},
			wantNotified: []Changes{{Added: []Lesson{inWeek}}},
		},
		{
			name:          "new week is published without a change alert",
			fetched:       []Lesson{kept, outOfWeek},
			wantApplied:   Changes{NewWeekLessons: []Lesson{outOfWeek}},
			wantCreated:   []time.Time{mondayOf(outOfWeek.StartAt)},
			wantPublished: []weekPublication{{weekID: "week", lessons: []Lesson{outOfWeek}}},
		},
		{
			name:          "mixed lessons are split by week",
			fetched:       []Lesson{kept, inWeek, outOfWeek},
			wantApplied:   Changes{NewWeekLessons: []Lesson{outOfWeek}, Added: []Lesson{inWeek}},
			wantNotified:  []Changes{{NewWeekLessons: []Lesson{outOfWeek}, Added: []Lesson{inWeek}}},
			wantCreated:   []time.Time{mondayOf(outOfWeek.StartAt)},
			wantPublished: []weekPublication{{weekID: "week", lessons: []Lesson{outOfWeek}}},
		},
		{
			name:        "all new weeks are published in chronological order",
			fetched:     []Lesson{kept, lastWeek, secondLesson, outOfWeek},
			wantApplied: Changes{NewWeekLessons: []Lesson{lastWeek, secondLesson, outOfWeek}},
			wantCreated: []time.Time{mondayOf(outOfWeek.StartAt), mondayOf(lastWeek.StartAt)},
			wantPublished: []weekPublication{
				{weekID: "week", lessons: []Lesson{outOfWeek, secondLesson}},
				{weekID: "week", lessons: []Lesson{lastWeek}},
			},
		},
		{
			name:      "create week error stops sync before apply",
			fetched:   []Lesson{kept, outOfWeek},
			createErr: errBoom,
			wantErr:   errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{stored: []Lesson{kept}}
			weeks := &fakeWeekRepo{noWeeksFrom: outOfWeek.StartAt, createErr: tt.createErr}
			n := &fakeNotifier{}
			s := NewSyncer(store, weeks, &fakeFetcher{lessons: tt.fetched}, n)

			err := s.Sync(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sync() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(store.applied, tt.wantApplied) {
				t.Errorf("Apply got %+v, want %+v", store.applied, tt.wantApplied)
			}
			if !reflect.DeepEqual(n.notified, tt.wantNotified) {
				t.Errorf("notified = %+v, want %+v", n.notified, tt.wantNotified)
			}
			if !reflect.DeepEqual(weeks.created, tt.wantCreated) {
				t.Errorf("created weeks = %v, want %v", weeks.created, tt.wantCreated)
			}
			if !reflect.DeepEqual(n.published, tt.wantPublished) {
				t.Errorf("published weeks = %+v, want %+v", n.published, tt.wantPublished)
			}
		})
	}
}

func TestAddEndShift(t *testing.T) {
	testTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	emptyLesson := Lesson{}

	tests := []struct {
		name      string
		subject   string
		kind      Kind
		wantEndAt time.Time
	}{
		{
			name:      "known lesson shifted +5m",
			subject:   "Алгоритмизация и программирование",
			kind:      KindPractice,
			wantEndAt: testTime.Add(5 * time.Minute),
		},
		{
			name:      "unknown lessons unchanged",
			subject:   "Test lesson",
			kind:      KindLecture,
			wantEndAt: testTime,
		},
		{
			name:      "known lesson shifted -20m",
			subject:   "Физическая культура",
			kind:      KindPractice,
			wantEndAt: testTime.Add(-20 * time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lessons := []Lesson{emptyLesson, {
				EndAt:   testTime,
				Subject: tt.subject,
				Kind:    tt.kind,
			}, emptyLesson}

			addEndShift(lessons)

			if !lessons[0].Equal(emptyLesson) || !lessons[2].Equal(emptyLesson) {
				t.Fatalf("affected other lessons = %+v, want = {%+v, ..., %+v}", lessons, emptyLesson, emptyLesson)
			}
			if !lessons[1].EndAt.Equal(tt.wantEndAt) {
				t.Fatalf("endAt = %s, want %s", lessons[1].EndAt.Format("02.01.2006 15:04:05"), tt.wantEndAt.Format("02.01.2006 15:04:05"))
			}
		})
	}
}
