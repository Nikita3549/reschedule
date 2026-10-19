package main

import (
	"context"

	"reschedule/internal/config"
	"reschedule/internal/parser"
)

func main() {
	cfg := config.LoadConfig()
	p := parser.New(cfg.ParserConfig)
	p.Run(context.Background())
}
