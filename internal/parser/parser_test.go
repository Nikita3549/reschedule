package parser

import (
	"os"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/schedule.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	lessons, err := parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got, want := len(lessons), 47; got != want {
		t.Fatalf("len(lessons) = %d, want %d", got, want)
	}

	at := func(day, clock string) time.Time {
		ts, err := time.ParseInLocation("02.01.2006 15:04", day+" "+clock, minsk)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}

	tests := []struct {
		name string
		want Lesson
	}{
		{
			name: "quotes and M15",
			want: Lesson{
				StartAt:  at("12.10.2026", "08:30"),
				Subject:  `Доп. виды обучения "Русский язык как иностранный"`,
				Kind:     KindPractice,
				Room:     "43",
				Location: LocationM15,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "parentheses inside subject",
			want: Lesson{
				StartAt:  at("12.10.2026", "10:10"),
				Subject:  "Белорусский язык (проф. лексика)",
				Kind:     KindPractice,
				Room:     "309",
				Location: LocationM5,
				Teacher:  "Толочко Дарья Андреевна",
			},
		},
		{
			name: "shorsa",
			want: Lesson{
				StartAt:  at("14.10.2026", "09:30"),
				Subject:  "Физическая культура",
				Kind:     KindPractice,
				Room:     "Спортивный комплекс",
				Location: LocationShorsa,
				Teacher:  "Омелюсик Ольга Валентиновна, Платонова Елена Павловна, Федосюк Инга Валерьевна, Федюк Кристина Евгеньевна",
			},
		},
		{
			name: "subgroups merged",
			want: Lesson{
				StartAt:  at("14.10.2026", "12:05"),
				Subject:  "Иностранный язык",
				Kind:     KindPractice,
				Room:     "3 подгр. — 303 ауд., 4 подгр. — 310 ауд.",
				Location: LocationOboynaya,
				Teacher:  "3 подгр. — Ларина Татьяна Викторовна, 4 подгр. — Вавилова Наталья Николаевна",
			},
		},
		{
			name: "lab",
			want: Lesson{
				StartAt:  at("15.10.2026", "10:10"),
				Subject:  "Компьютерная графика",
				Kind:     KindLab,
				Room:     "403",
				Location: LocationM5,
				Teacher:  "Шемаров Александр Александрович",
			},
		},
		{
			name: "unknown kind",
			want: Lesson{
				StartAt:  at("15.10.2026", "15:15"),
				Subject:  "Информационный час",
				Kind:     "",
				Room:     "309",
				Location: LocationM5,
				Teacher:  "Ярошевич Андрей Олегович",
			},
		},
		{
			name: "self study online with nested parentheses",
			want: Lesson{
				StartAt:  at("17.10.2026", "15:15"),
				Subject:  "Высшая математика",
				Kind:     KindSelfStudy,
				Room:     "",
				Location: LocationOnline,
				Teacher:  "Гапеева Ирина Александровна",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.EndAt = tt.want.StartAt.Add(lessonDuration)
			for _, l := range lessons {
				if l.StartAt.Equal(tt.want.StartAt) && l.Subject == tt.want.Subject {
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
