package watcher

import (
	"context"
	"log"
	"math/rand/v2"
	"time"
)

const (
	DefaultInterval = 5 * time.Minute
	DefaultSpread   = 2 * time.Minute
)

type Syncer interface {
	Sync(context.Context) error
}

type Watcher struct {
	syncer   Syncer
	interval time.Duration
	spread   time.Duration
}

func New(s Syncer) *Watcher {
	return &Watcher{
		syncer:   s,
		interval: DefaultInterval,
		spread:   DefaultSpread,
	}
}

func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTimer(withJitter(w.interval, w.spread))
	defer t.Stop()

	for {
		select {
		case <-t.C:
			err := w.syncer.Sync(ctx)
			if err != nil {
				log.Printf("watcher sync: %v", err)
			}
		case <-ctx.Done():
			return
		}

		t.Reset(withJitter(w.interval, w.spread))
	}
}

func withJitter(base, spread time.Duration) time.Duration {
	if spread <= 0 {
		return base
	}
	return base - spread + rand.N(spread*2)
}
