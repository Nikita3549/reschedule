package parser

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"reschedule/internal/config"
	"reschedule/internal/lesson"
)

func TestFetchSchedule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/schedule" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "testdata/schedule.html")
	}))
	t.Cleanup(server.Close)

	p := New(&config.ParserConfig{URL: server.URL + "/schedule"})
	got, err := p.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}
	if len(got) != 47 {
		t.Fatalf("Fetch() returned %d lessons, want 47", len(got))
	}
	start := time.Date(2026, 10, 14, 12, 5, 0, 0, lesson.TZ)
	if !slices.ContainsFunc(got, func(l lesson.Lesson) bool {
		return l.StartAt.Equal(start) && l.Subject == "Иностранный язык" &&
			l.Room == "3 подгр. — 303 ауд., 4 подгр. — 310 ауд." && l.ID != ""
	}) {
		t.Error("Fetch() did not return the merged foreign language lesson")
	}
}

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/schedule.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got, want := len(got), 47; got != want {
		t.Fatalf("len(got) = %d, want %d", got, want)
	}

	ids := make(map[string]struct{}, len(got))
	for _, l := range got {
		if l.ID == "" {
			t.Fatalf("empty ID for %+v", l)
		}
		if _, ok := ids[l.ID]; ok {
			t.Fatalf("duplicate ID %s for %+v", l.ID, l)
		}
		ids[l.ID] = struct{}{}
	}

	at := func(day, clock string) time.Time {
		ts, err := time.ParseInLocation("02.01.2006 15:04", day+" "+clock, lesson.TZ)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}

	tests := []struct {
		name string
		want lesson.Lesson
	}{
		{
			name: "quotes and M15",
			want: lesson.Lesson{
				StartAt:  at("12.10.2026", "08:30"),
				Subject:  `Доп. виды обучения "Русский язык как иностранный"`,
				Kind:     lesson.KindPractice,
				Room:     "43",
				Location: lesson.LocationM15,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "parentheses inside subject",
			want: lesson.Lesson{
				StartAt:  at("12.10.2026", "10:10"),
				Subject:  "Белорусский язык (проф. лексика)",
				Kind:     lesson.KindPractice,
				Room:     "309",
				Location: lesson.LocationM5,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "shorsa",
			want: lesson.Lesson{
				StartAt:  at("14.10.2026", "09:30"),
				Subject:  "Физическая культура",
				Kind:     lesson.KindPractice,
				Room:     "Спортивный комплекс",
				Location: lesson.LocationShorsa,
				Teacher:  "Омелюсик Ольга Валентиновна, Платонова Елена Павловна, Федосюк Инга Валерьевна, Федюк Кристина Евгеньевна",
			},
		},
		{
			name: "subgroups merged",
			want: lesson.Lesson{
				StartAt:  at("14.10.2026", "12:05"),
				Subject:  "Иностранный язык",
				Kind:     lesson.KindPractice,
				Room:     "3 подгр. — 303 ауд., 4 подгр. — 310 ауд.",
				Location: lesson.LocationOboynaya,
				Teacher:  "3 подгр. — Ларина Татьяна Викторовна, 4 подгр. — Вавилова Наталья Николаевна",
			},
		},
		{
			name: "lab",
			want: lesson.Lesson{
				StartAt:  at("15.10.2026", "10:10"),
				Subject:  "Компьютерная графика",
				Kind:     lesson.KindLab,
				Room:     "403",
				Location: lesson.LocationM5,
				Teacher:  "Шемаров Александр Александрович",
			},
		},
		{
			name: "unknown kind",
			want: lesson.Lesson{
				StartAt:  at("15.10.2026", "15:15"),
				Subject:  "Информационный час",
				Kind:     "",
				Room:     "309",
				Location: lesson.LocationM5,
				Teacher:  "Ярошевич Андрей Олегович",
			},
		},
		{
			name: "self study online with nested parentheses",
			want: lesson.Lesson{
				StartAt:  at("17.10.2026", "15:15"),
				Subject:  "Высшая математика",
				Kind:     lesson.KindSelfStudy,
				Room:     "",
				Location: lesson.LocationOnline,
				Teacher:  "Гапеева Ирина Александровна",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.EndAt = tt.want.StartAt.Add(lessonDuration)
			for _, l := range got {
				if l.StartAt.Equal(tt.want.StartAt) && l.Subject == tt.want.Subject {
					l.ID = ""
					if l != tt.want {
						t.Errorf("got  %+v\nwant %+v", l, tt.want)
					}
					return
				}
			}
			t.Errorf("lesson %s at %s not found", tt.want.Subject, tt.want.StartAt)
		})
	}
}

func TestLessonID(t *testing.T) {
	start := time.Date(2026, 10, 12, 8, 30, 0, 0, lesson.TZ)
	l := lesson.Lesson{StartAt: start, Subject: "Физика", Room: "309"}

	sameInstant := l
	sameInstant.StartAt = start.UTC()
	sameInstant.Room = "410"
	if lessonID(l) != lessonID(sameInstant) {
		t.Error("lessonID depends on time zone or room, want only instant and subject")
	}

	otherTime := l
	otherTime.StartAt = start.Add(time.Minute)
	if lessonID(l) == lessonID(otherTime) {
		t.Error("lessonID equal for different start times")
	}

	otherSubject := l
	otherSubject.Subject = "Химия"
	if lessonID(l) == lessonID(otherSubject) {
		t.Error("lessonID equal for different subjects")
	}
}
