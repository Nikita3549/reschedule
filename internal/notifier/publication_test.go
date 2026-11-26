package notifier

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"

	"reschedule/internal/db/dbtest"
	"reschedule/internal/lesson"
	"reschedule/internal/user"
	"reschedule/internal/week"
	"reschedule/internal/weekmsg"
)

type scheduleFetcher struct {
	lessons []lesson.Lesson
}

func (f *scheduleFetcher) Fetch(_ context.Context) ([]lesson.Lesson, error) {
	return f.lessons, nil
}

func TestWeeklyPublication(t *testing.T) {
	ctx := t.Context()
	db := dbtest.Tx(t)
	users := user.NewRepo(db)
	subscriber := user.User{ID: 42, ChatID: 99}
	if err := users.Save(ctx, subscriber); err != nil {
		t.Fatal(err)
	}

	type request struct {
		method string
		msgID  string
		text   string
	}
	requests := make(chan request, 8)
	var published atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method, ok := strings.CutPrefix(r.URL.Path, "/bot1:test/")
		if !ok || r.ParseMultipartForm(1<<20) != nil || r.FormValue("chat_id") != "99" {
			t.Error("invalid Telegram request", r.URL.Path)
			http.Error(w, "invalid Telegram request", http.StatusBadRequest)
			return
		}
		call := request{method: method, msgID: r.FormValue("message_id"), text: r.FormValue("text")}
		select {
		case requests <- call:
		default:
			t.Error("too many Telegram requests")
			http.Error(w, "too many requests", http.StatusBadRequest)
			return
		}
		switch method {
		case "sendMessage":
			id := int32(900)
			if strings.Contains(call.text, "Расписание на неделю") {
				id = 316 + published.Add(1)
			}
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, id)
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		case "pinChatMessage", "unpinChatMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	telegram, err := bot.New("1:test", bot.WithServerURL(api.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatal(err)
	}
	messages := weekmsg.NewRepo(db)
	n := &Notifier{bot: telegram, users: users, weekMsgs: messages}
	weeks := week.NewRepo(db)
	lessons := lesson.NewRepo(db)
	fetcher := &scheduleFetcher{}
	s := lesson.NewSyncer(lessons, weeks, fetcher, n)

	first := lessonAt(12, 8, 30, "Матан")
	day := time.Now().In(lesson.TZ).AddDate(0, 0, 7)
	first.StartAt = time.Date(day.Year(), day.Month(), day.Day(), 8, 30, 0, 0, lesson.TZ)
	first.EndAt = first.StartAt.Add(80 * time.Minute)
	next := first
	next.ID, next.Subject = "physics", "Физика"
	next.StartAt = first.StartAt.AddDate(0, 0, 7)
	next.EndAt = first.EndAt.AddDate(0, 0, 7)
	later := next
	later.ID, later.Subject = "geometry", "Геометрия"
	later.StartAt = next.StartAt.AddDate(0, 0, 7)
	later.EndAt = next.EndAt.AddDate(0, 0, 7)
	added := next
	added.ID, added.Subject = "programming", "Программирование"
	added.StartAt = next.StartAt.Add(2 * time.Hour)
	added.EndAt = next.EndAt.Add(2 * time.Hour)
	hidden := added
	hidden.ID, hidden.Subject = "hidden", "Информационный час"
	changed := next
	changed.Room = "771"

	checkRequests := func(name string, want ...request) {
		t.Helper()
		var got []request
		for len(requests) > 0 {
			got = append(got, <-requests)
		}
		editCount := 0
		for editCount < len(got) && got[editCount].method == "editMessageText" {
			editCount++
		}
		slices.SortFunc(got[:editCount], func(a, b request) int { return strings.Compare(a.msgID, b.msgID) })
		if !slices.Equal(got, want) {
			t.Fatalf("%s: Telegram requests = %+v, want %+v", name, got, want)
		}
	}
	sync := func(name string, schedule []lesson.Lesson, want ...request) {
		t.Helper()
		fetcher.lessons = schedule
		if err := s.Sync(ctx); err != nil {
			t.Fatalf("%s: Sync() = %v", name, err)
		}
		checkRequests(name, want...)
	}

	sync("bootstrap", []lesson.Lesson{first})
	sync("publish two weeks", []lesson.Lesson{first, later, next},
		request{method: "sendMessage", text: formatWeek([]lesson.Lesson{next})},
		request{method: "sendMessage", text: formatWeek([]lesson.Lesson{later})},
	)
	sync("update existing week", []lesson.Lesson{first, changed, added, hidden, later},
		request{method: "editMessageText", msgID: "317", text: formatWeek([]lesson.Lesson{changed, added})},
		request{method: "sendMessage", text: formatChanged([]lesson.Change{{Old: next, New: changed}})},
		request{method: "sendMessage", text: formatAdded([]lesson.Lesson{added})},
	)
	weekAt := func(date time.Time) string {
		t.Helper()
		w, err := weeks.ByDate(ctx, date)
		if err != nil {
			t.Fatal(err)
		}
		return w.ID
	}
	nextID, laterID := weekAt(next.StartAt), weekAt(later.StartAt)
	checkMessage := func(weekID string, msgID int, state weekmsg.PinState) {
		t.Helper()
		got, err := messages.ListByWeekID(ctx, weekID)
		if err != nil {
			t.Fatal(err)
		}
		want := weekmsg.Message{WeekID: weekID, UserID: subscriber.ID, ChatID: subscriber.ChatID, MsgID: msgID, PinState: state}
		if !slices.Equal(got, []weekmsg.Message{want}) {
			t.Errorf("messages for %s = %+v, want [%+v]", weekID, got, want)
		}
	}
	checkMessage(nextID, 317, weekmsg.Unpinned)
	checkMessage(laterID, 318, weekmsg.Unpinned)

	if err := n.RotateWeekPins(ctx, nextID); err != nil {
		t.Fatal(err)
	}
	checkRequests("pin published schedule", request{method: "pinChatMessage", msgID: "317"})
	checkMessage(nextID, 317, weekmsg.Pinned)
	if err := n.RotateWeekPins(ctx, laterID); err != nil {
		t.Fatal(err)
	}
	checkRequests("rotate pinned schedules",
		request{method: "pinChatMessage", msgID: "318"},
		request{method: "unpinChatMessage", msgID: "317"},
	)
	checkMessage(nextID, 317, weekmsg.Unpinned)
	checkMessage(laterID, 318, weekmsg.Pinned)
	if err := n.RotateWeekPins(ctx, laterID); err != nil {
		t.Fatal(err)
	}
	checkRequests("repeat pin rotation")

	updated := changed
	updated.Teacher = "Новый преподаватель"
	sync("update two published weeks", []lesson.Lesson{first, updated, added, hidden},
		request{method: "editMessageText", msgID: "317", text: formatWeek([]lesson.Lesson{updated, added})},
		request{method: "editMessageText", msgID: "318", text: "На этой неделе нет занятий"},
		request{method: "sendMessage", text: formatCanceled([]lesson.Lesson{later})},
		request{method: "sendMessage", text: formatChanged([]lesson.Change{{Old: changed, New: updated}})},
	)
	sync("unchanged schedule", []lesson.Lesson{first, updated, added, hidden})
	checkMessage(nextID, 317, weekmsg.Unpinned)
	checkMessage(laterID, 318, weekmsg.Pinned)

	stored, err := lessons.ListFrom(ctx, first.StartAt)
	if err != nil {
		t.Fatal(err)
	}
	hidden.Hide = true
	wantLessons := []lesson.Lesson{first, updated, added, hidden}
	slices.SortFunc(stored, func(a, b lesson.Lesson) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(wantLessons, func(a, b lesson.Lesson) int { return strings.Compare(a.ID, b.ID) })
	if !slices.EqualFunc(stored, wantLessons, lesson.Lesson.Equal) {
		t.Errorf("stored lessons = %+v, want %+v", stored, wantLessons)
	}
}
