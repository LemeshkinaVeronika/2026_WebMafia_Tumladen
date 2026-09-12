package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/webmafia/tumladan/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := worker.New(ctx, worker.LoadConfig())
	if err != nil {
		log.Fatalf("init event worker: %v", err)
	}
	if err := application.Run(ctx); err != nil {
		log.Fatalf("run event worker: %v", err)
	}
}
