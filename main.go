package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsByte/MarkovBotGo/backend"
	"github.com/itsByte/MarkovBotGo/bot"
)

const (
	persistTimer time.Duration = 600 * time.Second
	unloadTimer  time.Duration = 6 * time.Hour
)

func main() {
	flag.Parse()

	t := make(backend.Tables)
	go func() {
		ticker := time.NewTicker(persistTimer)
		defer ticker.Stop()
		for {
			<-ticker.C
			slog.Debug("Executing persistence routine")
			backend.Tables.Persist(t)
		}
	}()

	go func() {
		ticker := time.NewTicker(unloadTimer)
		defer ticker.Stop()
		for {
			<-ticker.C
			slog.Debug("Executing unload routine")
			backend.Tables.UnloadOld(t)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		slog.Info("Exiting gracefully")
		backend.Tables.Persist(t)
		os.Exit(0)
	}()
	slog.Info("Bot starting...")
	slog.Info("Options:", "ChainOrder", *backend.ChainOrder, "Chattiness", *bot.Chattiness, "ReplyChance", *bot.ReplyChance)
	bot.Init(t)
}
