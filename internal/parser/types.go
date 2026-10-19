package parser

import "time"

type LessonKind string

const (
	KindLecture   LessonKind = "lecture"
	KindSeminar   LessonKind = "seminar"
	KindPractice  LessonKind = "practice"
	KindLab       LessonKind = "lab"
	KindSelfStudy LessonKind = "self_study"
)

type LocationKind string

const (
	LocationM15      LocationKind = "м15"
	LocationM5       LocationKind = "м5"
	LocationOboynaya LocationKind = "Обойная 7"
	LocationOnline   LocationKind = "УСР"
	LocationShorsa   LocationKind = "Щорса 5а"
)

type Lesson struct {
	ID       string       `json:"id"`
	StartAt  time.Time    `json:"startAt"`
	EndAt    time.Time    `json:"endAt"`
	Subject  string       `json:"subject"`
	Kind     LessonKind   `json:"kind"`
	Room     string       `json:"room"`
	Location LocationKind `json:"location"`
	Teacher  string       `json:"teacher"`
}
