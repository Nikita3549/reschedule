package digest

import (
	"context"
	"errors"
	"testing"
	"time"

	"reschedule/internal/lessons"
)

type fakeLister struct {
	ls       []lessons.Lesson
	err      error
	from, to time.Time
}

func (f *fakeLister) ListBetween(_ context.Context, from, to time.Time) ([]lessons.Lesson, error) {
	f.from, f.to = from, to
	return f.ls, f.err
}

type fakeNotifier struct {
	calls int
	ls    []lessons.Lesson
	err   error
}

func (f *fakeNotifier) NotifyDaily(_ context.Context, ls []lessons.Lesson) error {
	f.calls++
	f.ls = ls
	return f.err
}

func newSender(l Lister, n Notifier, now time.Time) *Sender {
	s := New(l, n)
	s.now = func() time.Time { return now }
	return s
}

func TestNextRun(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before hour",
			now:  time.Date(2026, 11, 5, 10, 0, 0, 0, lessons.TZ),
			want: time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ),
		},
		{
			name: "exactly at hour",
			now:  time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ),
			want: time.Date(2026, 11, 6, 20, 0, 0, 0, lessons.TZ),
		},
		{
			name: "after hour",
			now:  time.Date(2026, 11, 5, 21, 0, 0, 0, lessons.TZ),
			want: time.Date(2026, 11, 6, 20, 0, 0, 0, lessons.TZ),
		},
		{
			name: "date differs from utc",
			now:  time.Date(2026, 11, 5, 22, 30, 0, 0, time.UTC),
			want: time.Date(2026, 11, 6, 20, 0, 0, 0, lessons.TZ),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRun(tt.now, DefaultHour); !got.Equal(tt.want) {
				t.Errorf("nextRun(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestTomorrow(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "same date in utc",
			now:  time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ),
			want: time.Date(2026, 11, 6, 0, 0, 0, 0, lessons.TZ),
		},
		{
			name: "date differs from utc",
			now:  time.Date(2026, 11, 5, 22, 30, 0, 0, time.UTC),
			want: time.Date(2026, 11, 7, 0, 0, 0, 0, lessons.TZ),
		},
		{
			name: "end of month",
			now:  time.Date(2026, 11, 30, 20, 0, 0, 0, lessons.TZ),
			want: time.Date(2026, 12, 1, 0, 0, 0, 0, lessons.TZ),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tomorrow(tt.now); !got.Equal(tt.want) {
				t.Errorf("tomorrow(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestSendRequestsNextDayFromCurrentTime(t *testing.T) {
	ls := []lessons.Lesson{{ID: "a"}}
	l := &fakeLister{ls: ls}
	n := &fakeNotifier{}
	now := time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ)
	s := New(l, n)
	s.now = func() time.Time { return now }

	for _, day := range []int{6, 7} {
		if err := s.send(context.Background()); err != nil {
			t.Fatalf("send() = %v", err)
		}

		from := time.Date(2026, 11, day, 0, 0, 0, 0, lessons.TZ)
		if !l.from.Equal(from) || !l.to.Equal(from.AddDate(0, 0, 1)) {
			t.Errorf("ListBetween(%v, %v), want day %d", l.from, l.to, day)
		}

		now = now.AddDate(0, 0, 1)
	}

	if len(n.ls) != len(ls) {
		t.Errorf("NotifyDaily lessons = %v, want %v", n.ls, ls)
	}
}

func TestSendSkipsNotify(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name    string
		lister  *fakeLister
		wantErr error
	}{
		{
			name:   "no lessons",
			lister: &fakeLister{},
		},
		{
			name:    "list failed",
			lister:  &fakeLister{err: errBoom},
			wantErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &fakeNotifier{}
			s := newSender(tt.lister, n, time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ))

			if err := s.send(context.Background()); !errors.Is(err, tt.wantErr) {
				t.Errorf("send() = %v, want %v", err, tt.wantErr)
			}
			if n.calls != 0 {
				t.Errorf("NotifyDaily calls = %d, want 0", n.calls)
			}
		})
	}
}

func TestSendReturnsNotifyError(t *testing.T) {
	errBoom := errors.New("boom")
	l := &fakeLister{ls: []lessons.Lesson{{ID: "a"}}}
	s := newSender(l, &fakeNotifier{err: errBoom}, time.Date(2026, 11, 5, 20, 0, 0, 0, lessons.TZ))

	if err := s.send(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("send() = %v, want %v", err, errBoom)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := &fakeNotifier{}
	s := New(&fakeLister{}, n)

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return on cancelled context")
	}

	if n.calls != 0 {
		t.Errorf("NotifyDaily calls = %d, want 0", n.calls)
	}
}
