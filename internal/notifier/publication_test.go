package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	sent := make(chan string, 4)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bot1:test/getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test"}}`))
		case "/bot1:test/sendMessage":
			if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("chat_id") != "99" {
				http.Error(w, "invalid Telegram request", http.StatusBadRequest)
				return
			}
			select {
			case sent <- r.FormValue("text"):
			default:
				http.Error(w, "too many messages", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":317,"chat":{"id":99,"type":"private"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	telegram, err := bot.New("1:test", bot.WithServerURL(api.URL))
	if err != nil {
		t.Fatal(err)
	}
	n := &Notifier{bot: telegram, users: users, weekMsgs: weekmsg.NewRepo(db)}
	weeks := week.NewRepo(db)
	fetcher := &scheduleFetcher{}
	s := lesson.NewSyncer(lesson.NewRepo(db), weeks, fetcher, n)

	first := lessonAt(12, 8, 30, "Матан")
	first.StartAt = time.Now().In(lesson.TZ).AddDate(0, 0, 7).Truncate(time.Minute)
	first.EndAt = first.StartAt.Add(80 * time.Minute)
	next := first
	next.ID, next.Subject = "physics", "Физика"
	next.StartAt = first.StartAt.AddDate(0, 0, 7)
	next.EndAt = first.EndAt.AddDate(0, 0, 7)
	added := next
	added.ID, added.Subject = "programming", "Программирование"

	sync := func(name string, schedule []lesson.Lesson, wantText ...string) {
		t.Helper()
		fetcher.lessons = schedule
		if err := s.Sync(ctx); err != nil {
			t.Fatalf("%s: Sync() = %v", name, err)
		}
		if got := len(sent); got != len(wantText) {
			t.Fatalf("%s: sent %d messages, want %d", name, got, len(wantText))
		}
		for _, want := range wantText {
			if got := <-sent; !strings.Contains(got, want) {
				t.Errorf("%s: message %q does not contain %q", name, got, want)
			}
		}
	}

	sync("bootstrap", []lesson.Lesson{first})
	sync("new week", []lesson.Lesson{first, next}, "Расписание на неделю")
	sync("new lesson in existing week", []lesson.Lesson{first, next, added}, "Добавлена пара")
	sync("unchanged schedule", []lesson.Lesson{first, next, added})

	w, err := weeks.ByDate(ctx, next.StartAt)
	if err != nil {
		t.Fatal(err)
	}
	var messages []weekmsg.Message
	if err := db.Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	want := weekmsg.Message{WeekID: w.ID, UserID: subscriber.ID, ChatID: subscriber.ChatID, MsgID: 317}
	if len(messages) != 1 || messages[0] != want {
		t.Errorf("week_messages = %+v, want [%+v]", messages, want)
	}
}
