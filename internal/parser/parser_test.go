package parser

import (
	"os"
	"testing"
	"time"

	"reschedule/internal/lessons"
)

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
		ts, err := time.ParseInLocation("02.01.2006 15:04", day+" "+clock, lessons.TZ)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}

	tests := []struct {
		name string
		want lessons.Lesson
	}{
		{
			name: "quotes and M15",
			want: lessons.Lesson{
				StartAt:  at("12.10.2026", "08:30"),
				Subject:  `Доп. виды обучения "Русский язык как иностранный"`,
				Kind:     lessons.KindPractice,
				Room:     "43",
				Location: lessons.LocationM15,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "parentheses inside subject",
			want: lessons.Lesson{
				StartAt:  at("12.10.2026", "10:10"),
				Subject:  "Белорусский язык (проф. лексика)",
				Kind:     lessons.KindPractice,
				Room:     "309",
				Location: lessons.LocationM5,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "shorsa",
			want: lessons.Lesson{
				StartAt:  at("14.10.2026", "09:30"),
				Subject:  "Физическая культура",
				Kind:     lessons.KindPractice,
				Room:     "Спортивный комплекс",
				Location: lessons.LocationShorsa,
				Teacher:  "Омелюсик Ольга Валентиновна, Платонова Елена Павловна, Федосюк Инга Валерьевна, Федюк Кристина Евгеньевна",
			},
		},
		{
			name: "subgroups merged",
			want: lessons.Lesson{
				StartAt:  at("14.10.2026", "12:05"),
				Subject:  "Иностранный язык",
				Kind:     lessons.KindPractice,
				Room:     "3 подгр. — 303 ауд., 4 подгр. — 310 ауд.",
				Location: lessons.LocationOboynaya,
				Teacher:  "3 подгр. — Ларина Татьяна Викторовна, 4 подгр. — Вавилова Наталья Николаевна",
			},
		},
		{
			name: "lab",
			want: lessons.Lesson{
				StartAt:  at("15.10.2026", "10:10"),
				Subject:  "Компьютерная графика",
				Kind:     lessons.KindLab,
				Room:     "403",
				Location: lessons.LocationM5,
				Teacher:  "Шемаров Александр Александрович",
			},
		},
		{
			name: "unknown kind",
			want: lessons.Lesson{
				StartAt:  at("15.10.2026", "15:15"),
				Subject:  "Информационный час",
				Kind:     "",
				Room:     "309",
				Location: lessons.LocationM5,
				Teacher:  "Ярошевич Андрей Олегович",
			},
		},
		{
			name: "self study online with nested parentheses",
			want: lessons.Lesson{
				StartAt:  at("17.10.2026", "15:15"),
				Subject:  "Высшая математика",
				Kind:     lessons.KindSelfStudy,
				Room:     "",
				Location: lessons.LocationOnline,
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
	start := time.Date(2026, 10, 12, 8, 30, 0, 0, lessons.TZ)
	l := lessons.Lesson{StartAt: start, Subject: "Физика", Room: "309"}

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
