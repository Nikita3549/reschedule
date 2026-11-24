package dbtest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"reschedule/internal/config"
	"reschedule/internal/db"
)

func testDSN(t *testing.T) (dsn string, explicit bool) {
	t.Helper()

	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn, true
	}

	if err := godotenv.Load(filepath.Join(moduleRoot(t), ".env")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("load .env: %v", err)
	}

	var cfg config.DBConfig
	if errs := config.ParseConfig(&cfg); len(errs) > 0 {
		t.Fatalf("parse db config: %v", errs)
	}
	return cfg.DSN(), false
}

func Tx(t *testing.T) *gorm.DB {
	t.Helper()

	dsn, explicit := testDSN(t)
	gormDB, err := db.Open(dsn)
	if err != nil {
		if !explicit {
			t.Skipf("db is unavailable: %v", err)
		}
		t.Fatalf("open db: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	paths, err := filepath.Glob(filepath.Join(moduleRoot(t), "migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	slices.Sort(paths)

	stmts := make([]string, 0, len(paths)+2)
	stmts = append(
		stmts,
		"CREATE SCHEMA dbtest",
		"SET LOCAL search_path TO dbtest",
	)
	for _, path := range paths {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(path), err)
		}
		stmts = append(stmts, string(migration))
	}

	tx := gormDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })

	for _, stmt := range stmts {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	return tx
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
