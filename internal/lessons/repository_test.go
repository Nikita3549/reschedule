package lessons

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"reschedule/internal/config"
	"reschedule/internal/db"
)

func testDSN(t *testing.T) (dsn string, explicit bool) {
	t.Helper()

	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn, true
	}

	if err := godotenv.Load("../../.env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("load .env: %v", err)
	}

	var cfg config.DBConfig
	if errs := config.ParseConfig(&cfg); len(errs) > 0 {
		t.Fatalf("parse db config: %v", errs)
	}
	return cfg.DSN(), false
}

func newTestRepo(t *testing.T) *Repo {
	t.Helper()

	dsn, explicit := testDSN(t)
	gormDB, err := db.Open(dsn)
	if err != nil {
		if !explicit {
			t.Skipf("db is unavailable: %v", err)
		}
		t.Fatalf("open db: %v", err)
	}

	migration, err := os.ReadFile("../../migrations/000001_create_lessons.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	tx := gormDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })

	for _, stmt := range []string{
		"CREATE SCHEMA repo_test",
		"SET LOCAL search_path TO repo_test",
		string(migration),
	} {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	return NewRepo(tx)
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

	if err := r.CreateMany(ctx, []Lesson{testLesson("a"), testLesson("b")}); err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	t.Run("FilterNew drops existing", func(t *testing.T) {
		got, err := r.FilterNew(ctx, []Lesson{testLesson("a"), testLesson("c"), testLesson("b"), testLesson("d")})
		if err != nil {
			t.Fatalf("FilterNew: %v", err)
		}
		if want := []string{"c", "d"}; !slices.Equal(ids(got), want) {
			t.Errorf("FilterNew ids = %v, want %v", ids(got), want)
		}
	})

	t.Run("FilterNew empty input", func(t *testing.T) {
		got, err := r.FilterNew(ctx, nil)
		if err != nil {
			t.Fatalf("FilterNew: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("FilterNew = %v, want empty", got)
		}
	})

	t.Run("CreateMany ignores duplicates", func(t *testing.T) {
		changed := testLesson("a")
		changed.Room = "999"
		if err := r.CreateMany(ctx, []Lesson{changed, testLesson("e")}); err != nil {
			t.Fatalf("CreateMany: %v", err)
		}

		var got []Lesson
		if err := r.db.WithContext(ctx).Order("id").Find(&got).Error; err != nil {
			t.Fatalf("select: %v", err)
		}
		if want := []string{"a", "b", "e"}; !slices.Equal(ids(got), want) {
			t.Fatalf("ids = %v, want %v", ids(got), want)
		}
		if got[0].Room != "309" {
			t.Errorf("Room = %q, want existing row untouched", got[0].Room)
		}
		if !got[0].StartAt.Equal(testLesson("a").StartAt) {
			t.Errorf("StartAt = %v, want %v", got[0].StartAt, testLesson("a").StartAt)
		}
	})

	t.Run("CreateMany empty input", func(t *testing.T) {
		if err := r.CreateMany(ctx, nil); err != nil {
			t.Errorf("CreateMany(nil) = %v, want nil", err)
		}
	})
}
