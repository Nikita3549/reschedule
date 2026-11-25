package notifier

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"reschedule/internal/lesson"
)

func lessonAt(day, hour, minute int, subject string) lesson.Lesson {
	start := time.Date(2026, 10, day, hour, minute, 0, 0, lesson.TZ)
	return lesson.Lesson{
		ID:       subject,
		StartAt:  start.UTC(),
		EndAt:    start.Add(80 * time.Minute).UTC(),
		Subject:  subject,
		Kind:     lesson.KindLecture,
		Room:     "309",
		Location: lesson.LocationM5,
		Teacher:  "Иванов Иван Иванович",
	}
}

func TestFormatAdded(t *testing.T) {
	tests := []struct {
		name    string
		lessons []lesson.Lesson
		want    string
	}{
		{
			name: "empty",
		},
		{
			name:    "single lesson",
			lessons: []lesson.Lesson{lessonAt(12, 8, 30, "Матан")},
			want: "🆕 *Добавлена пара*\n\n" +
				"📅 *Понедельник, 12 октября*\n\n" +
				"🔹 *08:30 – 09:50* \\| *Матан*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович",
		},
		{
			name: "one day sorted",
			lessons: []lesson.Lesson{
				lessonAt(12, 10, 5, "Физика"),
				lessonAt(12, 8, 30, "Матан"),
			},
			want: "🆕 *Добавлены пары*\n\n" +
				"📅 *Понедельник, 12 октября*\n\n" +
				"🔹 *08:30 – 09:50* \\| *Матан*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович\n\n" +
				"🔹 *10:05 – 11:25* \\| *Физика*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович",
		},
		{
			name: "several days",
			lessons: []lesson.Lesson{
				lessonAt(13, 8, 30, "Физика"),
				lessonAt(12, 8, 30, "Матан"),
			},
			want: "🗓 *Расписание на 12\\.10 – 13\\.10*\n\n" +
				"📅 *Понедельник, 12 октября*\n\n" +
				"🔹 *08:30 – 09:50* \\| *Матан*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович\n\n\\-\\-\\-\n\n" +
				"📅 *Вторник, 13 октября*\n\n" +
				"🔹 *08:30 – 09:50* \\| *Физика*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAdded(tt.lessons)
			if got != tt.want {
				t.Errorf("formatAdded() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestFormatAddedEscapesAndOmitsEmptyRoom(t *testing.T) {
	l := lessonAt(12, 8, 30, "Матем. анализ (часть 1)")
	l.Location = lesson.LocationOnline
	l.Room = ""

	got := formatAdded([]lesson.Lesson{l})

	for _, want := range []string{"*Матем\\. анализ \\(часть 1\\)*", "— 📍 Онлайн\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAdded() = %q, want substring %q", got, want)
		}
	}
}

func TestFormattersLimitOversizedBlock(t *testing.T) {
	old := lessonAt(12, 8, 30, strings.Repeat("А", maxMessageLen))
	moved := old
	moved.Room = "101"
	ls := []lesson.Lesson{old}

	tests := []struct {
		name string
		msg  string
	}{
		{"added", formatAdded(ls)},
		{"canceled", formatCanceled(ls)},
		{"daily", formatDaily(ls)},
		{"week", formatWeek(ls)},
		{"changed", formatChanged([]lesson.Change{{Old: old, New: moved}})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := utf8.RuneCountInString(tt.msg); n > maxMessageLen {
				t.Errorf("message has %d runes, max %d", n, maxMessageLen)
			}
			if !strings.HasSuffix(tt.msg, limitNotice) {
				t.Error("message is missing the truncation notice")
			}
		})
	}
}

func TestFormatCanceled(t *testing.T) {
	got := formatCanceled([]lesson.Lesson{lessonAt(12, 8, 30, "Матан")})

	want := "❌ *Отменена пара*\n\n" +
		"📅 *Понедельник, 12 октября*\n\n" +
		"🔹 *08:30 – 09:50* \\| ~Матан~\n" +
		"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович"
	if got != want {
		t.Errorf("formatCanceled() = %q, want %q", got, want)
	}
}

func TestFormatDaily(t *testing.T) {
	tests := []struct {
		name    string
		lessons []lesson.Lesson
		want    string
	}{
		{
			name: "no lessons",
		},
		{
			name: "sorted lessons",
			lessons: []lesson.Lesson{
				lessonAt(12, 10, 5, "Физика"),
				lessonAt(12, 8, 30, "Матан"),
			},
			want: "🗓 *Расписание на завтра:*\n\n" +
				"📅 *Понедельник, 12 октября*\n\n" +
				"🔹 *08:30 – 09:50* \\| *Матан*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович\n\n" +
				"🔹 *10:05 – 11:25* \\| *Физика*\n" +
				"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDaily(tt.lessons)
			if got != tt.want {
				t.Errorf("formatDaily() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestFormatWeek(t *testing.T) {
	got := formatWeek([]lesson.Lesson{
		lessonAt(13, 8, 30, "Физика"),
		lessonAt(12, 8, 30, "Матан"),
	})

	want := "🗓 *Расписание на неделю*\n\n" +
		"📅 *Понедельник, 12 октября*\n\n" +
		"🔹 *08:30 – 09:50* \\| *Матан*\n" +
		"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович\n\n\\-\\-\\-\n\n" +
		"📅 *Вторник, 13 октября*\n\n" +
		"🔹 *08:30 – 09:50* \\| *Физика*\n" +
		"— 📖 Лекция\n— 📍 ул\\. м5, ауд\\. 309\n— 👨🏼‍🏫 Иванов Иван Иванович"
	if got != want {
		t.Errorf("formatWeek() = %q, want %q", got, want)
	}

	if got := formatWeek(nil); got != "" {
		t.Errorf("formatWeek(nil) = %q, want empty string", got)
	}
}

func TestFormatWeekTruncatesAtDayBoundary(t *testing.T) {
	first := strings.Repeat("А", 2200)
	second := strings.Repeat("Б", 2200)
	msg := formatWeek([]lesson.Lesson{
		lessonAt(12, 8, 30, first),
		lessonAt(13, 8, 30, second),
	})
	if n := utf8.RuneCountInString(msg); n > maxMessageLen {
		t.Errorf("message has %d runes, max %d", n, maxMessageLen)
	}
	if !strings.Contains(msg, first) || strings.Contains(msg, "Вторник") {
		t.Error("message should contain the first day in full and omit the second day")
	}
	if !strings.HasSuffix(msg, "⚠️ Превышен лимит сообщения") {
		t.Error("message is missing the truncation notice")
	}
}

func TestFormatChanged(t *testing.T) {
	old := lessonAt(12, 8, 30, "Матан")
	moved := old
	moved.Room = "101"
	moved.Teacher = "Петров Петр Петрович"

	unchanged := lessonAt(13, 8, 30, "Физика")

	tests := []struct {
		name    string
		changes []lesson.Change
		want    string
	}{
		{
			name: "empty",
		},
		{
			name:    "only changed fields",
			changes: []lesson.Change{{Old: old, New: moved}},
			want: "✏️ *Изменения в расписании*\n\n" +
				"*Пн, 12 октября* `08:30 – 09:50` *Матан*\n" +
				"Аудитория: ~309~ → *101*\n" +
				"Преподаватель: ~Иванов Иван Иванович~ → *Петров Петр Петрович*",
		},
		{
			name:    "no visible diff",
			changes: []lesson.Change{{Old: unchanged, New: unchanged}},
		},
		{
			name: "empty value as dash",
			changes: []lesson.Change{{Old: old, New: func() lesson.Lesson {
				l := old
				l.Room = ""
				return l
			}()}},
			want: "✏️ *Изменения в расписании*\n\n" +
				"*Пн, 12 октября* `08:30 – 09:50` *Матан*\n" +
				"Аудитория: ~309~ → *—*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChanged(tt.changes)
			if got != tt.want {
				t.Errorf("formatChanged() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestLessonBlock(t *testing.T) {
	selfStudy := lessonAt(12, 20, 10, "Прикладная информатика")
	selfStudy.Kind = lesson.KindSelfStudy
	selfStudy.Location = lesson.LocationOnline
	selfStudy.Room = ""

	noKind := lessonAt(12, 15, 15, "Информационный час")
	noKind.Kind = ""

	subgroups := lessonAt(12, 12, 5, "Иностранный язык")
	subgroups.Kind = lesson.KindPractice
	subgroups.Location = lesson.LocationOboynaya
	subgroups.Room = "3 подгр. — 303 ауд., 4 подгр. — 310 ауд."
	subgroups.Teacher = "3 подгр. — Ларина Татьяна Викторовна, 4 подгр. — Вавилова Наталья Николаевна"

	tests := []struct {
		name   string
		lesson lesson.Lesson
		want   string
	}{
		{
			name:   "self study",
			lesson: selfStudy,
			want: "🟧 *20:10 – 21:30* \\| *Прикладная информатика \\(УСР🏠\\)*\n" +
				"— 💻 УСР\n" +
				"— 📍 Онлайн\n" +
				"— 👨🏼‍🏫 Иванов Иван Иванович",
		},
		{
			name:   "no kind",
			lesson: noKind,
			want: "🔹 *15:15 – 16:35* \\| *Информационный час*\n" +
				"— 📍 ул\\. м5, ауд\\. 309\n" +
				"— 👨🏼‍🏫 Иванов Иван Иванович",
		},
		{
			name:   "subgroups",
			lesson: subgroups,
			want: "🔹 *12:05 – 13:25* \\| *Иностранный язык*\n" +
				"— 📝 Практика\n" +
				"— 📍 ул\\. Обойная 7, ауд\\. 3 подгр\\. — 303 ауд\\., 4 подгр\\. — 310 ауд\\.\n" +
				"— 👥 3 подгр\\. — Ларина Татьяна Викторовна, 4 подгр\\. — Вавилова Наталья Николаевна",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lessonBlock(tt.lesson, "*"); got != tt.want {
				t.Errorf("lessonBlock() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestTeacherIcon(t *testing.T) {
	tests := []struct {
		teacher string
		want    string
	}{
		{"Толочко Дарья Андреевна", "👩🏼‍🏫"},
		{"Воробьева Елена Михайловна", "👩🏼‍🏫"},
		{"Горбачев Дмитрий Владимирович", "👨🏼‍🏫"},
		{"Минченков Юрий Владимирович", "👨🏼‍🏫"},
		{"Гапеева Ирина Александровна", "👩🏼‍🏫"},
		{"Колотков Алексей Владимирович", "👨🏼‍🏫"},
		{"Ярошевич Андрей Олегович", "👨🏼‍🏫"},
		{"Омелюсик Ольга Валентиновна, Платонова Елена Павловна, Федосюк Инга Валерьевна, Федюк Кристина Евгеньевна", "👥"},
		{"Ларина Татьяна Викторовна, Вавилова Наталья Николаевна", "👥"},
		{"Авдей Ольга Васильевна", "👩🏼‍🏫"},
		{"Шемаров Александр Александрович", "👨🏼‍🏫"},
		{"Бригадин Денис Петрович", "👨🏼‍🏫"},
		{"Самусевич Владимир Петрович", "👨🏼‍🏫"},
		{"Рыбакова Элла Вадимовна", "👩🏼‍🏫"},
		{"Кузьмина Анна Ильинична", "👩🏼‍🏫"},
		{"Иванов И.И.", "👤"},
	}

	for _, tt := range tests {
		t.Run(tt.teacher, func(t *testing.T) {
			if got := teacherIcon(tt.teacher); got != tt.want {
				t.Errorf("teacherIcon(%q) = %q, want %q", tt.teacher, got, tt.want)
			}
		})
	}
}
