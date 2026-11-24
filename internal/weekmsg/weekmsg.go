package weekmsg

type Message struct {
	WeekID string `gorm:"column:week_id;primaryKey"`
	UserID int64  `gorm:"column:user_id;primaryKey"`
	MsgID  int    `gorm:"column:message_id"`
	ChatID int64  `gorm:"column:chat_id"`
}

func (Message) TableName() string {
	return "week_messages"
}
