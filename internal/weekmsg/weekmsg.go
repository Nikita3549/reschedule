package weekmsg

type PinState int16

const (
	Unpinned PinState = iota
	PinPending
	Pinned
)

type Message struct {
	WeekID   string   `gorm:"column:week_id;primaryKey"`
	UserID   int64    `gorm:"column:user_id;primaryKey"`
	MsgID    int      `gorm:"column:message_id"`
	ChatID   int64    `gorm:"column:chat_id"`
	PinState PinState `gorm:"column:pin_state"`
}

func (Message) TableName() string {
	return "week_messages"
}
