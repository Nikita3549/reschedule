package notifier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"reschedule/internal/lessons"
	"reschedule/internal/users"
)

const startReply = "Вы подписались на уведомления об изменениях в расписании."

type UserStorer interface {
	Save(ctx context.Context, u users.User) error
	List(ctx context.Context) ([]users.User, error)
}

type Notifier struct {
	bot   *bot.Bot
	users UserStorer
}

func New(token string, s UserStorer) (*Notifier, error) {
	n := &Notifier{users: s}

	b, err := bot.New(token, bot.WithMessageTextHandler("/start", bot.MatchTypeCommandStartOnly, n.handleStart))
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}
	n.bot = b

	return n, nil
}

func (n *Notifier) Run(ctx context.Context) {
	n.bot.Start(ctx)
}

func (n *Notifier) Notify(ctx context.Context, ch lessons.Changes) error {
	msgs := slices.Concat(formatCanceled(ch.Canceled), formatChanged(ch.Changed), formatAdded(ch.Added))
	if len(msgs) == 0 {
		return nil
	}

	us, err := n.users.List(ctx)
	if err != nil {
		return fmt.Errorf("notify: %w", err)
	}

	var errs []error
	for _, u := range us {
		for _, text := range msgs {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := n.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    u.ChatID,
				Text:      text,
				ParseMode: models.ParseModeMarkdown,
			})
			if err != nil {
				errs = append(errs, fmt.Errorf("send to user %d: %w", u.ID, err))
				break
			}
		}
	}

	return errors.Join(errs...)
}

func (n *Notifier) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg.From == nil {
		return
	}

	u := users.User{
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
