package week

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"reschedule/internal/db/dbtest"

	"gorm.io/gorm"
)

func saveTestWeeks(t *testing.T, db *gorm.DB, testDate time.Time) [3]*Week {
	t.Helper()

	testWeeks := [3]*Week{
		{
			StartAt: testDate,
		}, {
			StartAt: testDate.AddDate(0, 0, 7),
		}, {
			StartAt: testDate.AddDate(0, 0, 14),
		},
	}

	err := db.Create(testWeeks).Error
	if err != nil {
		t.Fatalf("Create test weeks: %v", err)
	}

	return testWeeks
}

func TestRepo_ByDate(t *testing.T) {
	db := dbtest.Tx(t)
	r := NewRepo(db)
	ctx := context.Background()

	testDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
	testWeeks := saveTestWeeks(t, db, testDate)

	tests := []struct {
		name    string
		date    time.Time
		want    *Week
		wantErr error
	}{
		{
			name: "inside second week",
			date: testDate.AddDate(0, 0, 10),
			want: testWeeks[1],
		},
		{
			name:    "no week",
			date:    testDate.AddDate(0, 0, 100),
			wantErr: ErrNotFound,
		},
		{
			name: "border between first and second weeks",
			date: testDate.AddDate(0, 0, 7),
			want: testWeeks[1],
		},
		{
			name: "left border first week",
			date: testDate,
			want: testWeeks[0],
		},
		{
			name: "right border last week",
			date: testDate.AddDate(0, 0, 21).Add(-1 * time.Second),
			want: testWeeks[len(testWeeks)-1],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.ByDate(ctx, tt.date)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ByDate(%s): %v, want = %v", tt.date.Format("2006-01-02 15:04:05"), err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if got.ID != tt.want.ID {
				t.Fatalf("ByDate(%s) = %+v, want = %+v", tt.date.Format("2006-01-02 15:04:05"), got, tt.want)
			}
		})
	}
}

func TestRepo_Next(t *testing.T) {
	db := dbtest.Tx(t)
	r := NewRepo(db)
	ctx := context.Background()

	testDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
	testWeeks := saveTestWeeks(t, db, testDate)

	tests := []struct {
		name    string
		week    *Week
		want    *Week
		wantErr error
	}{
		{
			name: "return second week next to first one",
			week: testWeeks[0],
			want: testWeeks[1],
		},
		{
			name: "return third week next to second one",
			week: testWeeks[1],
			want: testWeeks[2],
		},
		{
			name:    "last week",
			week:    testWeeks[len(testWeeks)-1],
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.Next(ctx, tt.week)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Next(%+v): %v, want = %v", tt.week, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if got.ID != tt.want.ID {
				t.Fatalf("Next(%+v) = %+v, want = %+v", tt.week, got, tt.want)
			}
		})
	}
}

func TestRepo_IDsByDates(t *testing.T) {
	db := dbtest.Tx(t)
	r := NewRepo(db)
	ctx := context.Background()

	testDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
	testWeeks := saveTestWeeks(t, db, testDate)

	tests := []struct {
		name  string
		dates []time.Time
		want  []string
	}{
		{
			name:  "2 inside",
			dates: []time.Time{testWeeks[0].StartAt, testWeeks[1].End().Add(-1 * time.Second)},
			want:  []string{testWeeks[0].ID, testWeeks[1].ID},
		},
		{
			name:  "1 inside, 1 outside",
			dates: []time.Time{testWeeks[0].StartAt.AddDate(0, 0, 3), testDate.AddDate(0, 0, 100)},
			want:  []string{testWeeks[0].ID, ""},
		},
		{
			name:  "outside boundaries",
			dates: []time.Time{testWeeks[0].StartAt.Add(-1 * time.Minute), testWeeks[len(testWeeks)-1].End()},
			want:  []string{"", ""},
		},
		{
			name:  "border between weeks",
			dates: []time.Time{testWeeks[1].StartAt},
			want:  []string{testWeeks[1].ID},
		},
		{
			name:  "empty weeks",
			dates: []time.Time{},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.IDsByDates(ctx, tt.dates)
			if err != nil {
				t.Fatalf("IDsByDates(%v): %v", tt.dates, err)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("IDsByDates(%v) = %v, want = %v", tt.dates, got, tt.want)
			}
		})
	}
}
