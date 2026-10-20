package lessons

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeFetcher struct {
	lessons []Lesson
	err     error
}

func (f *fakeFetcher) Fetch(context.Context) ([]Lesson, error) {
	return f.lessons, f.err
}

type fakeStore struct {
	fresh     []Lesson
	filterErr error
	createErr error

	filtered []Lesson
	created  []Lesson
	calls    []string
}

func (s *fakeStore) FilterNew(_ context.Context, lessons []Lesson) ([]Lesson, error) {
	s.calls = append(s.calls, "FilterNew")
	s.filtered = lessons
	return s.fresh, s.filterErr
}

func (s *fakeStore) CreateMany(_ context.Context, lessons []Lesson) error {
	s.calls = append(s.calls, "CreateMany")
	s.created = lessons
	return s.createErr
}

func TestSyncerSync(t *testing.T) {
	fetched := []Lesson{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	fresh := []Lesson{{ID: "c"}}
	errBoom := errors.New("boom")

	tests := []struct {
		name      string
		fetcher   *fakeFetcher
		store     *fakeStore
		wantErr   error
		wantCalls []string
	}{
		{
			name:      "creates only fresh lessons",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{fresh: fresh},
			wantCalls: []string{"FilterNew", "CreateMany"},
		},
		{
			name:      "fetch error stops sync",
			fetcher:   &fakeFetcher{err: errBoom},
			store:     &fakeStore{},
			wantErr:   errBoom,
			wantCalls: nil,
		},
		{
			name:      "filter error stops sync",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{filterErr: errBoom},
			wantErr:   errBoom,
			wantCalls: []string{"FilterNew"},
		},
		{
			name:      "create error is returned",
			fetcher:   &fakeFetcher{lessons: fetched},
			store:     &fakeStore{fresh: fresh, createErr: errBoom},
			wantErr:   errBoom,
			wantCalls: []string{"FilterNew", "CreateMany"},
		},
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
			if !reflect.DeepEqual(tt.store.filtered, fetched) {
				t.Errorf("FilterNew got %v, want %v", tt.store.filtered, fetched)
			}
			if !reflect.DeepEqual(tt.store.created, fresh) {
				t.Errorf("CreateMany got %v, want %v", tt.store.created, fresh)
			}
		})
	}
}
