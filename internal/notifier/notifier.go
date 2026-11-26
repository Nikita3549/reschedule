package notifier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

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
	ListByWeekID(ctx context.Context, weekID string) ([]weekmsg.Message, error)
	ListForPinRotation(ctx context.Context, weekID string) ([]weekmsg.Message, error)
	SetPinState(ctx context.Context, msg weekmsg.Message, state weekmsg.PinState) error
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
	_, err := n.broadcast(ctx, formatCanceled(ch.Canceled), formatChanged(ch.Changed), formatAdded(ch.Added))
	return err
}

func (n *Notifier) NotifyDaily(ctx context.Context, ls []lesson.Lesson) error {
	_, err := n.broadcast(ctx, formatDaily(ls))
	return err
}

func (n *Notifier) NotifyNoLessonsNextWeek(ctx context.Context) error {
	_, err := n.broadcast(ctx, "На следующей неделе нет пар.")
	return err
}

func (n *Notifier) RotateWeekPins(ctx context.Context, weekID string) error {
	msgs, err := n.weekMsgs.ListForPinRotation(ctx, weekID)
	if err != nil {
		return fmt.Errorf("list messages for pin rotation to week %s: %w", weekID, err)
	}

	var errs []error
	ready := make(map[int64]bool)
	for _, msg := range msgs {
		if msg.WeekID != weekID {
			continue
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := n.pinWeekMessage(ctx, msg); err != nil {
			errs = append(errs, err)
			ready[msg.ChatID] = false
		} else if _, exists := ready[msg.ChatID]; !exists {
			ready[msg.ChatID] = true
		}
	}
	if len(ready) == 0 {
		return fmt.Errorf("no published messages for week %s", weekID)
	}

	for _, msg := range msgs {
		if msg.WeekID == weekID {
			continue
		}
		ok, exists := ready[msg.ChatID]
		if !exists {
			errs = append(errs, fmt.Errorf("no published message for week %s in chat %d", weekID, msg.ChatID))
			ready[msg.ChatID] = false
		}
		if !ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := n.unpinWeekMessage(ctx, msg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (n *Notifier) pinWeekMessage(ctx context.Context, msg weekmsg.Message) error {
	if msg.PinState == weekmsg.Pinned {
		return nil
	}
	if msg.PinState == weekmsg.Unpinned {
		if err := n.savePinState(ctx, msg, weekmsg.PinPending); err != nil {
			return err
		}
	}
	ok, err := n.bot.PinChatMessage(ctx, &bot.PinChatMessageParams{
		ChatID:    msg.ChatID,
		MessageID: msg.MsgID,
	})
	if err != nil {
		return fmt.Errorf("pin message %d in chat %d: %w", msg.MsgID, msg.ChatID, err)
	}
	if !ok {
		return fmt.Errorf("Telegram did not pin message %d in chat %d", msg.MsgID, msg.ChatID)
	}
	return n.savePinState(ctx, msg, weekmsg.Pinned)
}

func (n *Notifier) unpinWeekMessage(ctx context.Context, msg weekmsg.Message) error {
	ok, err := n.bot.UnpinChatMessage(ctx, &bot.UnpinChatMessageParams{
		ChatID:    msg.ChatID,
		MessageID: msg.MsgID,
	})
	if err != nil {
		return fmt.Errorf("unpin message %d in chat %d: %w", msg.MsgID, msg.ChatID, err)
	}
	if !ok {
		return fmt.Errorf("Telegram did not unpin message %d in chat %d", msg.MsgID, msg.ChatID)
	}
	return n.savePinState(ctx, msg, weekmsg.Unpinned)
}

func (n *Notifier) savePinState(ctx context.Context, msg weekmsg.Message, state weekmsg.PinState) error {
	if err := n.weekMsgs.SetPinState(ctx, msg, state); err != nil {
		return fmt.Errorf("save pin state for message %d in chat %d: %w", msg.MsgID, msg.ChatID, err)
	}
	return nil
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

func (n *Notifier) EditWeek(ctx context.Context, chatID int64, msgID int, ls []lesson.Lesson) error {
	return n.editWeekMessage(ctx, chatID, msgID, formatWeek(ls))
}

func (n *Notifier) EditWeekMessages(ctx context.Context, weekID string, ls []lesson.Lesson) error {
	msgs, err := n.weekMsgs.ListByWeekID(ctx, weekID)
	if err != nil {
		return fmt.Errorf("list messages for week %s: %w", weekID, err)
	}

	text := formatWeek(ls)
	var errs []error
	for _, msg := range msgs {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		err := n.editWeekMessage(ctx, msg.ChatID, msg.MsgID, text)
		if err == nil || (errors.Is(err, bot.ErrorBadRequest) && strings.Contains(err.Error(), "message is not modified")) {
			continue
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (n *Notifier) editWeekMessage(ctx context.Context, chatID int64, msgID int, text string) error {
	if text == "" {
		text = "На этой неделе нет занятий"
	}

	_, err := n.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		return fmt.Errorf("edit week message %d in chat %d: %w", msgID, chatID, err)
	}
	return nil
}

func (n *Notifier) sendWeek(ctx context.Context, chatID int64, ls []lesson.Lesson) (int, error) {
	text := formatWeek(ls)
	if text == "" {
		return 0, nil
	}

	msg, err := n.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		return 0, fmt.Errorf("send week to chat %d: %w", chatID, err)
	}
	return msg.ID, nil
}

func (n *Notifier) broadcast(ctx context.Context, msgs ...string) ([]sentMessage, error) {
	if !slices.ContainsFunc(msgs, func(text string) bool { return text != "" }) {
		return nil, nil
	}

	us, err := n.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("notify: %w", err)
	}

	var sent []sentMessage
	for _, u := range us {
		for _, text := range msgs {
			if text == "" {
				continue
			}
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
