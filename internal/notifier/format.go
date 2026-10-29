package notifier

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"

	"reschedule/internal/lessons"
)

const maxMessageLen = 4000

var weekdays = [...]string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

var shortWeekdays = [...]string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}

var months = [...]string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

var kindNames = map[lessons.Kind]string{
	lessons.KindLecture:   "Лекция",
	lessons.KindSeminar:   "Семинар",
	lessons.KindPractice:  "Практика",
	lessons.KindLab:       "Лабораторная",
	lessons.KindSelfStudy: "Контролируемая УСР",
}

type day struct {
	date    time.Time
	lessons []lessons.Lesson
}

func formatAdded(ls []lessons.Lesson) []string {
	if len(ls) == 0 {
		return nil
	}

	days := groupByDay(ls)

	var header string
	switch {
	case len(ls) == 1:
		header = "🆕 *Добавлена пара*"
	case len(days) == 1:
		header = "🆕 *Добавлены пары*"
	default:
		from, to := days[0].date, days[len(days)-1].date
		header = fmt.Sprintf("🗓 *Расписание на %s – %s*", bot.EscapeMarkdown(from.Format("02.01")), bot.EscapeMarkdown(to.Format("02.01")))
	}

	return pack(header, dayBlocks(days, "*"))
}

func formatCanceled(ls []lessons.Lesson) []string {
	if len(ls) == 0 {
		return nil
	}

	header := "❌ *Отменены пары*"
	if len(ls) == 1 {
		header = "❌ *Отменена пара*"
	}

	return pack(header, dayBlocks(groupByDay(ls), "~"))
}

func formatChanged(changes []lessons.Change) []string {
	sorted := slices.SortedFunc(slices.Values(changes), func(a, b lessons.Change) int {
		return a.New.StartAt.Compare(b.New.StartAt)
	})

	blocks := make([]string, 0, len(sorted))
	for _, c := range sorted {
		if block := changeBlock(c); block != "" {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		return nil
	}

	return pack("✏️ *Изменения в расписании*", blocks)
}

func changeBlock(c lessons.Change) string {
	fields := []struct {
		name     string
		old, new string
	}{
		{"Время", timeRange(c.Old), timeRange(c.New)},
		{"Тип", kindNames[c.Old.Kind], kindNames[c.New.Kind]},
		{"Корпус", string(c.Old.Location), string(c.New.Location)},
		{"Аудитория", c.Old.Room, c.New.Room},
		{"Преподаватель", c.Old.Teacher, c.New.Teacher},
	}

	var diff []string
	for _, f := range fields {
		if f.old == f.new {
			continue
		}
		diff = append(diff, fmt.Sprintf("%s: ~%s~ → *%s*", f.name, orDash(f.old), orDash(f.new)))
	}
	if len(diff) == 0 {
		return ""
	}

	title := fmt.Sprintf("*%s* `%s` *%s*", bot.EscapeMarkdown(shortDate(c.New.StartAt)), timeRange(c.New), bot.EscapeMarkdown(c.New.Subject))
	return title + "\n" + strings.Join(diff, "\n")
}

func dayBlocks(days []day, subjectStyle string) []string {
	blocks := make([]string, 0, len(days))
	for _, d := range days {
		ls := make([]string, 0, len(d.lessons))
		for _, l := range d.lessons {
			ls = append(ls, lessonBlock(l, subjectStyle))
		}
		blocks = append(blocks, fmt.Sprintf("*%s*\n%s", bot.EscapeMarkdown(longDate(d.date)), strings.Join(ls, "\n\n")))
	}
	return blocks
}

func lessonBlock(l lessons.Lesson, subjectStyle string) string {
	block := fmt.Sprintf("`%s` %s%s%s", timeRange(l), subjectStyle, bot.EscapeMarkdown(l.Subject), subjectStyle)
	if details := lessonDetails(l); details != "" {
		block += "\n" + bot.EscapeMarkdown(details)
	}
	return block
}

func lessonDetails(l lessons.Lesson) string {
	place := string(l.Location)
	if l.Room != "" {
		place += ", ауд. " + l.Room
	}

	var parts []string
	for _, p := range []string{kindNames[l.Kind], place, l.Teacher} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

func groupByDay(ls []lessons.Lesson) []day {
	sorted := slices.SortedFunc(slices.Values(ls), func(a, b lessons.Lesson) int {
		return a.StartAt.Compare(b.StartAt)
	})

	var days []day
	for _, l := range sorted {
		start := l.StartAt.In(lessons.TZ)
		if n := len(days); n > 0 && sameDate(days[n-1].date, start) {
			days[n-1].lessons = append(days[n-1].lessons, l)
			continue
		}
		days = append(days, day{date: start, lessons: []lessons.Lesson{l}})
	}
	return days
}

func pack(header string, blocks []string) []string {
	var (
		msgs []string
		b    strings.Builder
	)
	b.WriteString(header)
	for _, block := range blocks {
		if b.Len() > 0 && utf8.RuneCountInString(b.String())+2+utf8.RuneCountInString(block) > maxMessageLen {
			msgs = append(msgs, b.String())
			b.Reset()
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(block)
	}
	return append(msgs, b.String())
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func timeRange(l lessons.Lesson) string {
	return l.StartAt.In(lessons.TZ).Format("15:04") + "–" + l.EndAt.In(lessons.TZ).Format("15:04")
}

func longDate(t time.Time) string {
	t = t.In(lessons.TZ)
	return fmt.Sprintf("%s, %d %s", weekdays[t.Weekday()], t.Day(), months[t.Month()])
}

func shortDate(t time.Time) string {
	t = t.In(lessons.TZ)
	return fmt.Sprintf("%s, %d %s", shortWeekdays[t.Weekday()], t.Day(), months[t.Month()])
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return bot.EscapeMarkdown(s)
}
