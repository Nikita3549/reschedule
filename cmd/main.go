package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"reschedule/internal/config"
	"reschedule/internal/db"
	"reschedule/internal/lessons"
	"reschedule/internal/parser"
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

	p := parser.New(cfg.ParserConfig)
	lessonsRepo := lessons.NewRepo(gormDB)

	s := lessons.NewSyncer(lessonsRepo, p)

	w := watcher.New(s)

	log.Println("bot is running")
	w.Run(ctx)
}
