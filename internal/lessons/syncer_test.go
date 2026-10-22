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
	stored   []Lesson
	listErr  error
	applyErr error

	applied Changes
	calls   []string
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

	all := []string{"ListFrom", "Apply"}

	tests := []struct {
		name      string
		fetcher   *fakeFetcher
		store     *fakeStore
		wantErr   error
		wantCalls []string
	}{
		{
			name:      "applies diff of future lessons",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{stored: stored},
			wantCalls: all,
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
			wantCalls: all[:1],
		},
		{
			name:      "apply error is returned",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{stored: stored, applyErr: errBoom},
			wantErr:   errBoom,
			wantCalls: all,
		},
	}

	want := Changes{
		Added:    []Lesson{added},
		Canceled: []Lesson{canceled},
		Changed:  []Change{{Old: old, New: changed}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSyncer(tt.store, tt.fetcher)

			err := s.Sync(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sync() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.store.calls, tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", tt.store.calls, tt.wantCalls)
			}
			if tt.wantErr != nil {
				return
			}
			if !reflect.DeepEqual(tt.store.applied, want) {
				t.Errorf("Apply got %+v, want %+v", tt.store.applied, want)
			}
		})
	}
}
