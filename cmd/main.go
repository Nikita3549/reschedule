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
	"reschedule/internal/lessons"
	"reschedule/internal/notifier"
	"reschedule/internal/parser"
	"reschedule/internal/users"
	"reschedule/internal/watcher"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadConfig()
	gormDB, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	usersRepo := users.NewRepo(gormDB)
	notifier, err := notifier.New(cfg.Token, usersRepo)
	if err != nil {
		log.Fatalf("create notifier: %v", err)
	}
	go notifier.Run(ctx)

	parser := parser.New(cfg.ParserConfig)
	lessonsRepo := lessons.NewRepo(gormDB)

	sender := digest.New(lessonsRepo, notifier)
	go sender.Run(ctx)

	syncer := lessons.NewSyncer(lessonsRepo, parser, notifier)

	watcher := watcher.New(syncer)

	log.Println("bot is running")
	watcher.Run(ctx)
}
