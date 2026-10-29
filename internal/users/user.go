package users

import "time"

type User struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	ChatID       int64     `gorm:"column:chat_id;not null"`
	Username     string    `gorm:"column:username;not null"`
	FirstName    string    `gorm:"column:first_name;not null"`
	LastName     string    `gorm:"column:last_name;not null"`
	LanguageCode string    `gorm:"column:language_code;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (User) TableName() string {
	return "users"
}
