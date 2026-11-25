package notifier

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"

	"reschedule/internal/db/dbtest"
	"reschedule/internal/user"
	"reschedule/internal/week"
	"reschedule/internal/weekmsg"
)

func TestRotateWeekPinsKeepsOldPinWhenReplacementFails(t *testing.T) {
	ctx := t.Context()
	db := dbtest.Tx(t)
	users := user.NewRepo(db)
	weeks := week.NewRepo(db)
	messages := weekmsg.NewRepo(db)
	monday := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	oldWeekID, err := weeks.Create(ctx, monday)
	if err != nil {
		t.Fatal(err)
	}
	nextWeekID, err := weeks.Create(ctx, monday.AddDate(0, 0, 14))
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []user.User{{ID: 1, ChatID: 11}, {ID: 2, ChatID: 22}} {
		if err := users.Save(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	old := []weekmsg.Message{
		{WeekID: oldWeekID, UserID: 1, ChatID: 11, MsgID: 101, PinState: weekmsg.Pinned},
		{WeekID: oldWeekID, UserID: 2, ChatID: 22, MsgID: 102, PinState: weekmsg.Pinned},
	}
	if err := messages.CreateMany(ctx, old); err != nil {
		t.Fatal(err)
	}

	var failPin atomic.Bool
	requests := make(chan string, 8)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method := strings.TrimPrefix(r.URL.Path, "/bot1:test/")
		if method == "getMe" {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test"}}`))
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		chatID, msgID := r.FormValue("chat_id"), r.FormValue("message_id")
		select {
		case requests <- method + ":" + chatID + ":" + msgID:
		default:
			http.Error(w, "too many requests", http.StatusBadRequest)
			return
		}
		if failPin.Load() && chatID == "11" && method == "pinChatMessage" {
			_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"try again"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	t.Cleanup(api.Close)
	telegram, err := bot.New("1:test", bot.WithServerURL(api.URL))
	if err != nil {
		t.Fatal(err)
	}
	n := &Notifier{bot: telegram, weekMsgs: messages}

	if err := n.RotateWeekPins(ctx, nextWeekID); err == nil || len(requests) != 0 {
		t.Fatalf("rotation without messages returned %v and made %d Telegram requests", err, len(requests))
	}
	next := []weekmsg.Message{
		{WeekID: nextWeekID, UserID: 1, ChatID: 11, MsgID: 201},
		{WeekID: nextWeekID, UserID: 2, ChatID: 22, MsgID: 202},
	}
	if err := messages.CreateMany(ctx, next); err != nil {
		t.Fatal(err)
	}

	checkRequests := func(wantPins []string, wantUnpin string) {
		t.Helper()
		var got []string
		for len(requests) > 0 {
			got = append(got, <-requests)
		}
		if len(got) != len(wantPins)+1 || got[len(got)-1] != wantUnpin {
			t.Fatalf("requests = %v, want pins %v followed by %s", got, wantPins, wantUnpin)
		}
		pins := got[:len(wantPins)]
		slices.Sort(pins)
		if !slices.Equal(pins, wantPins) {
			t.Errorf("pin requests = %v, want %v", pins, wantPins)
		}
	}
	checkStates := func(want ...weekmsg.PinState) {
		t.Helper()
		var got []weekmsg.PinState
		if err := db.WithContext(ctx).Model(&weekmsg.Message{}).Order("message_id").Pluck("pin_state", &got).Error; err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("message pin states = %v, want %v", got, want)
		}
	}

	failPin.Store(true)
	if err := n.RotateWeekPins(ctx, nextWeekID); err == nil || !strings.Contains(err.Error(), "pin message 201 in chat 11") {
		t.Fatalf("rotation with failed pin returned %v", err)
	}
	checkRequests([]string{"pinChatMessage:11:201", "pinChatMessage:22:202"}, "unpinChatMessage:22:102")
	checkStates(weekmsg.Pinned, weekmsg.Unpinned, weekmsg.PinPending, weekmsg.Pinned)

	failPin.Store(false)
	if err := n.RotateWeekPins(ctx, nextWeekID); err != nil {
		t.Fatal(err)
	}
	checkRequests([]string{"pinChatMessage:11:201"}, "unpinChatMessage:11:101")
	checkStates(weekmsg.Unpinned, weekmsg.Unpinned, weekmsg.Pinned, weekmsg.Pinned)
}
