package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"reschedule/internal/config"
	"reschedule/internal/db"
	"reschedule/internal/digest"
	"reschedule/internal/lesson"
	"reschedule/internal/notifier"
	"reschedule/internal/parser"
	"reschedule/internal/user"
	"reschedule/internal/watcher"
	"reschedule/internal/week"
	"reschedule/internal/weekmsg"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadConfig()
	gormDB, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	weekRepo := week.NewRepo(gormDB)
	weekMsgRepo := weekmsg.NewRepo(gormDB)
	userRepo := user.NewRepo(gormDB)
	lessonRepo := lesson.NewRepo(gormDB)

	notifier, err := notifier.New(
		cfg.Token,
		&notifier.Deps{
			Users:    userRepo,
			WeekMsgs: weekMsgRepo,
			Weeks:    weekRepo,
			Lessons:  lessonRepo,
		},
	)
	if err != nil {
		log.Fatalf("create notifier: %v", err)
	}
	go notifier.Run(ctx)

	parser := parser.New(cfg.ParserConfig)

	sender := digest.New(lessonRepo, notifier, weekRepo)
	go sender.Run(ctx)

	syncer := lesson.NewSyncer(lessonRepo, weekRepo, parser, notifier)

	watcher := watcher.New(syncer)

	log.Println("bot is running")
	watcher.Run(ctx)
}
