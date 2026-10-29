package notifier

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"reschedule/internal/lessons"
)

func lessonAt(day, hour, minute int, subject string) lessons.Lesson {
	start := time.Date(2026, 10, day, hour, minute, 0, 0, lessons.TZ)
	return lessons.Lesson{
		ID:       subject,
		StartAt:  start.UTC(),
		EndAt:    start.Add(80 * time.Minute).UTC(),
		Subject:  subject,
		Kind:     lessons.KindLecture,
		Room:     "309",
		Location: lessons.LocationM5,
		Teacher:  "Иванов И.И.",
	}
}

func TestFormatAdded(t *testing.T) {
	tests := []struct {
		name    string
		lessons []lessons.Lesson
		want    []string
	}{
		{
			name: "empty",
		},
		{
			name:    "single lesson",
			lessons: []lessons.Lesson{lessonAt(12, 8, 30, "Матан")},
			want: []string{"🆕 *Добавлена пара*\n\n" +
				"*Понедельник, 12 октября*\n" +
				"`08:30–09:50` *Матан*\n" +
				"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\."},
		},
		{
			name: "one day sorted",
			lessons: []lessons.Lesson{
				lessonAt(12, 10, 5, "Физика"),
				lessonAt(12, 8, 30, "Матан"),
			},
			want: []string{"🆕 *Добавлены пары*\n\n" +
				"*Понедельник, 12 октября*\n" +
				"`08:30–09:50` *Матан*\n" +
				"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\.\n\n" +
				"`10:05–11:25` *Физика*\n" +
				"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\."},
		},
		{
			name: "several days",
			lessons: []lessons.Lesson{
				lessonAt(13, 8, 30, "Физика"),
				lessonAt(12, 8, 30, "Матан"),
			},
			want: []string{"🗓 *Расписание на 12\\.10 – 13\\.10*\n\n" +
				"*Понедельник, 12 октября*\n" +
				"`08:30–09:50` *Матан*\n" +
				"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\.\n\n" +
				"*Вторник, 13 октября*\n" +
				"`08:30–09:50` *Физика*\n" +
				"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAdded(tt.lessons)
			if strings.Join(got, "\n---\n") != strings.Join(tt.want, "\n---\n") {
				t.Errorf("formatAdded() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestFormatAddedEscapesAndOmitsEmptyRoom(t *testing.T) {
	l := lessonAt(12, 8, 30, "Матем. анализ (часть 1)")
	l.Location = lessons.LocationOnline
	l.Room = ""

	got := formatAdded([]lessons.Lesson{l})[0]

	for _, want := range []string{"*Матем\\. анализ \\(часть 1\\)*", "Лекция · УСР · Иванов"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAdded() = %q, want substring %q", got, want)
		}
	}
}

func TestFormatAddedSplitsLongMessages(t *testing.T) {
	var ls []lessons.Lesson
	for d := 1; d <= 30; d++ {
		for h := 8; h < 20; h += 2 {
			ls = append(ls, lessonAt(d, h, 0, strings.Repeat("Очень длинный предмет ", 2)))
		}
	}

	got := formatAdded(ls)

	if len(got) < 2 {
		t.Fatalf("formatAdded() returned %d messages, want split", len(got))
	}
	for i, msg := range got {
		if n := utf8.RuneCountInString(msg); n > maxMessageLen {
			t.Errorf("message %d has %d runes, max %d", i, n, maxMessageLen)
		}
		if i > 0 && !strings.HasPrefix(msg, "*") {
			t.Errorf("message %d does not start with a day header: %q", i, msg[:20])
		}
	}
}

func TestFormatCanceled(t *testing.T) {
	got := formatCanceled([]lessons.Lesson{lessonAt(12, 8, 30, "Матан")})

	want := "❌ *Отменена пара*\n\n" +
		"*Понедельник, 12 октября*\n" +
		"`08:30–09:50` ~Матан~\n" +
		"Лекция · м5, ауд\\. 309 · Иванов И\\.И\\."
	if len(got) != 1 || got[0] != want {
		t.Errorf("formatCanceled() = %q, want %q", got, want)
	}
}

func TestFormatChanged(t *testing.T) {
	old := lessonAt(12, 8, 30, "Матан")
	moved := old
	moved.Room = "101"
	moved.Teacher = "Петров П.П."

	unchanged := lessonAt(13, 8, 30, "Физика")

	tests := []struct {
		name    string
		changes []lessons.Change
		want    []string
	}{
		{
			name: "empty",
		},
		{
			name:    "only changed fields",
			changes: []lessons.Change{{Old: old, New: moved}},
			want: []string{"✏️ *Изменения в расписании*\n\n" +
				"*Пн, 12 октября* `08:30–09:50` *Матан*\n" +
				"Аудитория: ~309~ → *101*\n" +
				"Преподаватель: ~Иванов И\\.И\\.~ → *Петров П\\.П\\.*"},
		},
		{
			name:    "no visible diff",
			changes: []lessons.Change{{Old: unchanged, New: unchanged}},
		},
		{
			name: "empty value as dash",
			changes: []lessons.Change{{Old: old, New: func() lessons.Lesson {
				l := old
				l.Room = ""
				return l
			}()}},
			want: []string{"✏️ *Изменения в расписании*\n\n" +
				"*Пн, 12 октября* `08:30–09:50` *Матан*\n" +
				"Аудитория: ~309~ → *—*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChanged(tt.changes)
			if strings.Join(got, "\n---\n") != strings.Join(tt.want, "\n---\n") {
				t.Errorf("formatChanged() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
