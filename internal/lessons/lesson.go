package lessons

import "time"

var TZ = time.FixedZone("Europe/Minsk", 3*60*60)

type Kind string

const (
	KindLecture   Kind = "lecture"
	KindSeminar   Kind = "seminar"
	KindPractice  Kind = "practice"
	KindLab       Kind = "lab"
	KindSelfStudy Kind = "self_study"
)

type Location string

const (
	LocationM15      Location = "м15"
	LocationM5       Location = "м5"
	LocationOboynaya Location = "Обойная 7"
	LocationOnline   Location = "УСР"
	LocationShorsa   Location = "Щорса 5а"
)

type Lesson struct {
	ID       string    `gorm:"column:id;primaryKey"`
	StartAt  time.Time `gorm:"column:start_at;not null"`
	EndAt    time.Time `gorm:"column:end_at;not null"`
	Subject  string    `gorm:"column:subject;not null"`
	Kind     Kind      `gorm:"column:kind;not null"`
	Room     string    `gorm:"column:room;not null"`
	Location Location  `gorm:"column:location;not null"`
	Teacher  string    `gorm:"column:teacher;not null"`
}

func (Lesson) TableName() string {
	return "lessons"
}

type Change struct {
	Old Lesson
	New Lesson
}

type Changes struct {
	Added    []Lesson
	Canceled []Lesson
	Changed  []Change
}

func (c Changes) Empty() bool {
	return len(c.Added) == 0 && len(c.Canceled) == 0 && len(c.Changed) == 0
}
