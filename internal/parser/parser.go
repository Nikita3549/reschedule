package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"reschedule/internal/config"
	"reschedule/internal/lessons"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const lessonDuration = 80 * time.Minute

var kinds = map[string]lessons.Kind{
	"Лекция": lessons.KindLecture,
	"Практическое занятие": lessons.KindPractice,
	"Семинар":                 lessons.KindSeminar,
	"Лабораторная работа":     lessons.KindLab,
	"Контролируемая УСР (ДО)": lessons.KindSelfStudy,
}

var addresses = map[string]lessons.Location{
	"ул. Московская 15а": lessons.LocationM15,
	"ул. Московская 5":   lessons.LocationM5,
	"ул. Обойная 7":      lessons.LocationOboynaya,
}

type Parser struct {
	config *config.ParserConfig
}

func New(config *config.ParserConfig) *Parser {
	return &Parser{
		config,
	}
}

func (p *Parser) Fetch(ctx context.Context) ([]lessons.Lesson, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch schedule: unexpected status %s", resp.Status)
	}

	result, err := parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	return result, nil
}

type row struct {
	lesson   lessons.Lesson
	subgroup string
}

func parse(r io.Reader) ([]lessons.Lesson, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	table := find(doc, func(n *html.Node) bool {
		return n.DataAtom == atom.Table && hasClass(n, "table-schedule")
	})
	if table == nil {
		return nil, errors.New("schedule table not found")
	}
	tbody := find(table, func(n *html.Node) bool { return n.DataAtom == atom.Tbody })
	if tbody == nil {
		return nil, errors.New("schedule table has no tbody")
	}

	var (
		rows []row
		day  time.Time
	)
	for tr := range tbody.ChildNodes() {
		if tr.DataAtom != atom.Tr {
			continue
		}
		cells := elements(tr, atom.Td)

		if len(cells) == 1 && hasClass(cells[0], "head-date") {
			fields := strings.Fields(text(cells[0]))
			if len(fields) == 0 {
				return nil, errors.New("empty date header")
			}
			day, err = time.ParseInLocation("02.01.2006", fields[0], lessons.TZ)
			if err != nil {
				return nil, fmt.Errorf("parse date %q: %w", fields[0], err)
			}
			continue
		}

		if day.IsZero() {
			return nil, errors.New("lesson row before any date header")
		}
		rw, err := parseRow(day, cells)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", day.Format("02.01.2006"), err)
		}
		rows = append(rows, rw)
	}

	result := make([]lessons.Lesson, 0, len(rows))
	for i := 0; i < len(rows); {
		j := i + 1
		for j < len(rows) && sameLesson(rows[i], rows[j]) {
			j++
		}
		lesson := merge(rows[i:j])
		lesson.ID = lessonID(lesson)
		result = append(result, lesson)
		i = j
	}
	return result, nil
}

func parseRow(day time.Time, cells []*html.Node) (row, error) {
	if len(cells) != 4 {
		return row{}, fmt.Errorf("expected 4 cells, got %d", len(cells))
	}

	timeLines := lines(cells[0])
	if len(timeLines) == 0 {
		return row{}, errors.New("empty time cell")
	}
	clock, err := time.Parse("15:04", timeLines[0])
	if err != nil {
		return row{}, fmt.Errorf("parse time %q: %w", timeLines[0], err)
	}
	startAt := day.Add(time.Duration(clock.Hour())*time.Hour + time.Duration(clock.Minute())*time.Minute)

	var subgroup string
	if len(timeLines) > 1 {
		subgroup = strings.Join(timeLines[1:], " ")
	}

	subject, kind := splitKind(text(cells[1]))
	location, room := parseRoom(text(cells[3]))

	return row{
		lesson: lessons.Lesson{
			StartAt:  startAt,
			EndAt:    startAt.Add(lessonDuration),
			Subject:  subject,
			Kind:     kinds[kind],
			Room:     room,
			Location: location,
			Teacher:  text(cells[2]),
		},
		subgroup: subgroup,
	}, nil
}

func splitKind(s string) (subject, kind string) {
	if !strings.HasSuffix(s, ")") {
		return s, ""
	}
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[:i]), s[i+1 : len(s)-1]
			}
		}
	}
	return s, ""
}

func parseRoom(s string) (lessons.Location, string) {
	switch s {
	case "Онлайн, уч. портал":
		return lessons.LocationOnline, ""
	case "ул. Щорса,5а, Спортивный комплекс":
		return lessons.LocationShorsa, "Спортивный комплекс"
	}

	i := strings.LastIndex(s, ",")
	if i < 0 {
		return "", s
	}
	location, ok := addresses[strings.TrimSpace(s[:i])]
	if !ok {
		return "", s
	}
	return location, strings.TrimSpace(s[i+1:])
}

func lessonID(l lessons.Lesson) string {
	sum := sha256.Sum256([]byte(l.StartAt.UTC().Format(time.RFC3339) + "|" + l.Subject))
	return hex.EncodeToString(sum[:])
}

func sameLesson(a, b row) bool {
	return a.subgroup != "" && b.subgroup != "" &&
		a.lesson.StartAt.Equal(b.lesson.StartAt) &&
		a.lesson.Subject == b.lesson.Subject &&
		a.lesson.Kind == b.lesson.Kind
}

func merge(rows []row) lessons.Lesson {
	lesson := rows[0].lesson
	if rows[0].subgroup == "" {
		return lesson
	}

	rooms := make([]string, 0, len(rows))
	teachers := make([]string, 0, len(rows))
	for _, r := range rows {
		subgroup := strings.TrimSuffix(r.subgroup, ".") + "."
		if r.lesson.Room == "" {
			rooms = append(rooms, subgroup)
		} else {
			rooms = append(rooms, subgroup+" — "+r.lesson.Room+" ауд.")
		}
		teachers = append(teachers, subgroup+" — "+r.lesson.Teacher)
	}
	lesson.Room = strings.Join(rooms, ", ")
	lesson.Teacher = strings.Join(teachers, ", ")
	return lesson
}

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	for d := range n.Descendants() {
		if match(d) {
			return d
		}
	}
	return nil
}

func elements(n *html.Node, a atom.Atom) []*html.Node {
	var nodes []*html.Node
	for c := range n.ChildNodes() {
		if c.DataAtom == a {
			nodes = append(nodes, c)
		}
	}
	return nodes
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && slices.Contains(strings.Fields(attr.Val), class) {
			return true
		}
	}
	return false
}

func lines(n *html.Node) []string {
	var result []string
	for d := range n.Descendants() {
		if d.Type != html.TextNode {
			continue
		}
		if s := normalize(d.Data); s != "" {
			result = append(result, s)
		}
	}
	return result
}

func text(n *html.Node) string {
	return normalize(strings.Join(lines(n), " "))
}

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
