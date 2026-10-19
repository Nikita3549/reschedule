package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"reschedule/internal/config"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var minsk = time.FixedZone("Europe/Minsk", 3*60*60)

const lessonDuration = 80 * time.Minute

var kinds = map[string]LessonKind{
	"Лекция": KindLecture,
	"Практическое занятие": KindPractice,
	"Семинар":                 KindSeminar,
	"Лабораторная работа":     KindLab,
	"Контролируемая УСР (ДО)": KindSelfStudy,
}

var addresses = map[string]LocationKind{
	"ул. Московская 15а": LocationM15,
	"ул. Московская 5":   LocationM5,
	"ул. Обойная 7":      LocationOboynaya,
}

type Parser struct {
	config *config.ParserConfig
}

func New(config *config.ParserConfig) *Parser {
	return &Parser{
		config,
	}
}

func (p *Parser) Run(ctx context.Context) ([]Lesson, error) {
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

	lessons, err := parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	return lessons, nil
}

type row struct {
	lesson   Lesson
	subgroup string
}

func parse(r io.Reader) ([]Lesson, error) {
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
			day, err = time.ParseInLocation("02.01.2006", fields[0], minsk)
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

	lessons := make([]Lesson, 0, len(rows))
	for i := 0; i < len(rows); {
		j := i + 1
		for j < len(rows) && sameLesson(rows[i], rows[j]) {
			j++
		}
		lessons = append(lessons, merge(rows[i:j]))
		i = j
	}
	return lessons, nil
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
		lesson: Lesson{
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

func parseRoom(s string) (LocationKind, string) {
	switch s {
	case "Онлайн, уч. портал":
		return LocationOnline, ""
	case "ул. Щорса,5а, Спортивный комплекс":
		return LocationShorsa, "Спортивный комплекс"
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

func sameLesson(a, b row) bool {
	return a.subgroup != "" && b.subgroup != "" &&
		a.lesson.StartAt.Equal(b.lesson.StartAt) &&
		a.lesson.Subject == b.lesson.Subject &&
		a.lesson.Kind == b.lesson.Kind
}

func merge(rows []row) Lesson {
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
