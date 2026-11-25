package notifier

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"reschedule/internal/lesson"
	"reschedule/internal/user"
	"reschedule/internal/weekmsg"
)

const startReply = "Вы подписались на уведомления об изменениях в расписании."

type UserStorer interface {
	Save(ctx context.Context, u user.User) error
	List(ctx context.Context) ([]user.User, error)
}

type WeekMsgStorer interface {
	CreateMany(ctx context.Context, wm []weekmsg.Message) error
}

type Notifier struct {
	bot      *bot.Bot
	users    UserStorer
	weekMsgs WeekMsgStorer
}

type sentMessage struct {
	ID     int
	chatID int64
	userID int64
}

func New(token string, s UserStorer, wm WeekMsgStorer) (*Notifier, error) {
	n := &Notifier{users: s, weekMsgs: wm}

	b, err := bot.New(token, bot.WithMessageTextHandler("start", bot.MatchTypeCommandStartOnly, n.handleStart))
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}
	n.bot = b

	return n, nil
}

func (n *Notifier) Run(ctx context.Context) {
	n.bot.Start(ctx)
}

func (n *Notifier) Notify(ctx context.Context, ch lesson.Changes) error {
	_, err := n.broadcast(ctx, slices.Concat(formatCanceled(ch.Canceled), formatChanged(ch.Changed), formatAdded(ch.Added)))
	return err
}

func (n *Notifier) NotifyDaily(ctx context.Context, ls []lesson.Lesson) error {
	_, err := n.broadcast(ctx, formatDaily(ls))
	return err
}

func (n *Notifier) PublishWeek(ctx context.Context, weekID string, ls []lesson.Lesson) error {
	msgs, err := n.broadcast(ctx, formatWeek(ls))
	if err != nil {
		return fmt.Errorf("broadcast messages: %w", err)
	}

	var weekMsgs []weekmsg.Message
	for _, msg := range msgs {
		weekMsgs = append(weekMsgs, weekmsg.Message{
			WeekID: weekID,
			UserID: msg.userID,
			MsgID:  msg.ID,
			ChatID: msg.chatID,
		})
	}

	err = n.weekMsgs.CreateMany(ctx, weekMsgs)
	if err != nil {
		return fmt.Errorf("save weekMsgs %d: %w", len(weekMsgs), err)
	}

	return err
}

func (n *Notifier) sendWeek(ctx context.Context, chatID int64, ls []lesson.Lesson) ([]int, error) {
	msgs := formatWeek(ls)
	ids := make([]int, 0, len(msgs))
	for _, text := range msgs {
		m, err := n.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      text,
			ParseMode: models.ParseModeMarkdown,
		})
		if err != nil {
			return ids, fmt.Errorf("send week to chat %d: %w", chatID, err)
		}
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func (n *Notifier) broadcast(ctx context.Context, msgs []string) ([]sentMessage, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	us, err := n.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("notify: %w", err)
	}

	var sent []sentMessage
	for _, u := range us {
		for _, text := range msgs {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			msg, err := n.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    u.ChatID,
				Text:      text,
				ParseMode: models.ParseModeMarkdown,
			})
			if err != nil {
				if err = ctx.Err(); err != nil {
					return nil, err
				}
				log.Printf("send to user %d: %v", u.ID, err)
				break
			}

			sent = append(sent, sentMessage{
				ID:     msg.ID,
				chatID: u.ChatID,
				userID: u.ID,
			})
		}
	}

	return sent, nil
}

func (n *Notifier) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg.From == nil {
		return
	}

	u := user.User{
		ID:           msg.From.ID,
		ChatID:       msg.Chat.ID,
		Username:     msg.From.Username,
		FirstName:    msg.From.FirstName,
		LastName:     msg.From.LastName,
		LanguageCode: msg.From.LanguageCode,
	}
	if err := n.users.Save(ctx, u); err != nil {
		log.Printf("handle start: %v", err)
		return
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: startReply})
	if err != nil {
		log.Printf("reply start to user %d: %v", u.ID, err)
	}
}
