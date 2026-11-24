package lesson

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"reschedule/internal/db/dbtest"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()

	return NewRepo(dbtest.Tx(t))
}

func testLesson(id string) Lesson {
	start := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
	return Lesson{
		ID:       id,
		StartAt:  start,
		EndAt:    start.Add(80 * time.Minute),
		Subject:  "subject " + id,
		Kind:     KindLecture,
		Room:     "309",
		Location: LocationM5,
		Teacher:  "teacher",
	}
}

func ids(lessons []Lesson) []string {
	out := make([]string, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, l.ID)
	}
	return out
}

func TestRepo(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	if err := r.Apply(ctx, Changes{Added: []Lesson{testLesson("a"), testLesson("b")}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	t.Run("ListFrom filters by start_at", func(t *testing.T) {
		got, err := r.ListFrom(ctx, testLesson("a").StartAt.Add(time.Minute))
		if err != nil {
			t.Fatalf("ListFrom: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ListFrom = %v, want empty", ids(got))
		}

		got, err = r.ListFrom(ctx, testLesson("a").StartAt)
		if err != nil {
			t.Fatalf("ListFrom: %v", err)
		}
		slices.SortFunc(got, func(a, b Lesson) int { return strings.Compare(a.ID, b.ID) })
		if want := []string{"a", "b"}; !slices.Equal(ids(got), want) {
			t.Errorf("ListFrom ids = %v, want %v", ids(got), want)
		}
	})

	t.Run("ListBetween excludes upper bound", func(t *testing.T) {
		start := testLesson("a").StartAt

		got, err := r.ListBetween(ctx, start.Add(-time.Minute), start)
		if err != nil {
			t.Fatalf("ListBetween: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ListBetween = %v, want empty", ids(got))
		}

		got, err = r.ListBetween(ctx, start, start.Add(time.Minute))
		if err != nil {
			t.Fatalf("ListBetween: %v", err)
		}
		slices.SortFunc(got, func(a, b Lesson) int { return strings.Compare(a.ID, b.ID) })
		if want := []string{"a", "b"}; !slices.Equal(ids(got), want) {
			t.Errorf("ListBetween ids = %v, want %v", ids(got), want)
		}
	})

	t.Run("Apply ignores duplicate added", func(t *testing.T) {
		dup := testLesson("a")
		dup.Room = "999"
		if err := r.Apply(ctx, Changes{Added: []Lesson{dup, testLesson("e")}}); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		got := listAll(t, r)
		if want := []string{"a", "b", "e"}; !slices.Equal(ids(got), want) {
			t.Fatalf("ids = %v, want %v", ids(got), want)
		}
		if !got[0].Equal(testLesson("a")) {
			t.Errorf("got %+v, want existing row untouched", got[0])
		}
	})

	t.Run("Apply updates changed and deletes canceled", func(t *testing.T) {
		changed := testLesson("b")
		changed.Room = ""
		changed.Teacher = "new teacher"
		err := r.Apply(ctx, Changes{
			Changed:  []Change{{Old: testLesson("b"), New: changed}},
			Canceled: []Lesson{testLesson("e")},
		})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}

		got := listAll(t, r)
		if want := []string{"a", "b"}; !slices.Equal(ids(got), want) {
			t.Fatalf("ids = %v, want %v", ids(got), want)
		}
		if !got[1].Equal(changed) {
			t.Errorf("got %+v, want %+v", got[1], changed)
		}
	})

	t.Run("Apply empty changes", func(t *testing.T) {
		if err := r.Apply(ctx, Changes{}); err != nil {
			t.Errorf("Apply(empty) = %v, want nil", err)
		}
	})

	t.Run("ListBetween skips hidden", func(t *testing.T) {
		hidden := testLesson("h")
		hidden.Hide = true
		if err := r.Apply(ctx, Changes{Added: []Lesson{hidden}}); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		start := hidden.StartAt
		got, err := r.ListBetween(ctx, start, start.Add(time.Minute))
		if err != nil {
			t.Fatalf("ListBetween: %v", err)
		}
		if slices.Contains(ids(got), "h") {
			t.Errorf("ListBetween ids = %v, want without hidden", ids(got))
		}
	})
}

func listAll(t *testing.T, r *Repo) []Lesson {
	t.Helper()

	var lessons []Lesson
	if err := r.db.Order("id").Find(&lessons).Error; err != nil {
		t.Fatalf("select: %v", err)
	}
	return lessons
}
