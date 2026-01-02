package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsByte/tgmarkovgo/backend"
	"github.com/itsByte/tgmarkovgo/bot"
	"github.com/itsByte/tgmarkovgo/migrate"
)

const (
	persistTimer time.Duration = 600 * time.Second
	unloadTimer  time.Duration = 6 * time.Hour
)

func main() {
	flag.Parse()

	migrate.MigrateChains()

	chain, err := backend.BuildChain()
	if err != nil {
		slog.Error("Failed to build chain", "error", err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		slog.Info("Exiting gracefully")
		chain.Close()
		os.Exit(0)
	}()
	slog.Info("Bot starting...")
	slog.Info("Options:", "ChainOrder", *backend.ChainOrder, "Chattiness", *bot.Chattiness, "ReplyChance", *bot.ReplyChance)
	bot.Init(chain)
}
