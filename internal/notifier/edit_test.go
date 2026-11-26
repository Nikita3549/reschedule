package notifier

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-telegram/bot"

	"reschedule/internal/db/dbtest"
	"reschedule/internal/lesson"
	"reschedule/internal/user"
	"reschedule/internal/week"
	"reschedule/internal/weekmsg"
)

func TestEditWeekMessages(t *testing.T) {
	ctx := t.Context()
	db := dbtest.Tx(t)
	weeks := week.NewRepo(db)
	monday := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	weekID, err := weeks.Create(ctx, monday)
	if err != nil {
		t.Fatal(err)
	}
	otherWeekID, err := weeks.Create(ctx, monday.AddDate(0, 0, 7))
	if err != nil {
		t.Fatal(err)
	}
	users := user.NewRepo(db)
	for _, u := range []user.User{{ID: 1, ChatID: 11}, {ID: 2, ChatID: 22}, {ID: 3, ChatID: 33}} {
		if err := users.Save(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	messages := weekmsg.NewRepo(db)
	if err := messages.CreateMany(ctx, []weekmsg.Message{
		{WeekID: weekID, UserID: 1, ChatID: 11, MsgID: 101},
		{WeekID: weekID, UserID: 2, ChatID: 22, MsgID: 102, PinState: weekmsg.Pinned},
		{WeekID: weekID, UserID: 3, ChatID: 33, MsgID: 103},
		{WeekID: otherWeekID, UserID: 1, ChatID: 11, MsgID: 201, PinState: weekmsg.Pinned},
	}); err != nil {
		t.Fatal(err)
	}

	ls := []lesson.Lesson{lessonAt(16, 8, 30, "Физика")}
	wantText := formatWeek(ls)
	requests := make(chan string, 4)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot1:test/editMessageText" || r.ParseMultipartForm(1<<20) != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if got := r.FormValue("text"); got != wantText {
			t.Errorf("message text = %q, want %q", got, wantText)
		}
		if got := r.FormValue("parse_mode"); got != "MarkdownV2" {
			t.Errorf("parse mode = %q, want MarkdownV2", got)
		}
		msgID := r.FormValue("message_id")
		select {
		case requests <- r.FormValue("chat_id") + ":" + msgID:
		default:
			t.Error("too many edit requests")
		}
		switch msgID {
		case "101":
			_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked"}`))
		case "102":
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":103}}`))
		}
	}))
	t.Cleanup(api.Close)
	telegram, err := bot.New("1:test", bot.WithServerURL(api.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatal(err)
	}
	n := &Notifier{bot: telegram, weekMsgs: messages}
	err = n.EditWeekMessages(ctx, weekID, ls)
	if !errors.Is(err, bot.ErrorForbidden) || errors.Is(err, bot.ErrorBadRequest) {
		t.Errorf("EditWeekMessages() = %v, want only the forbidden error", err)
	}
	var got []string
	for len(requests) > 0 {
		got = append(got, <-requests)
	}
	slices.Sort(got)
	if want := []string{"11:101", "22:102", "33:103"}; !slices.Equal(got, want) {
		t.Errorf("edit requests = %v, want %v", got, want)
	}
}
