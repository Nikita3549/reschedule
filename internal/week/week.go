package week

import "time"

type Week struct {
	ID      string    `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	StartAt time.Time `gorm:"column:start_at;not null"`
}

func (Week) TableName() string {
	return "weeks"
}

func (w *Week) End() time.Time {
	return w.StartAt.AddDate(0, 0, 7)
}
