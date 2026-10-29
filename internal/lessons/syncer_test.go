package lessons

import (
	"context"
	"errors"
	"reflect"
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

type fakeNotifier struct {
	err      error
	notified []Changes
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
			s := NewSyncer(tt.store, tt.fetcher, n)

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
