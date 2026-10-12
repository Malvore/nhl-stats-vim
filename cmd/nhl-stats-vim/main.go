package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Malvore/nhl-stats-vim/internal/nhl"
	"github.com/Malvore/nhl-stats-vim/internal/server"
	"github.com/Malvore/nhl-stats-vim/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var err error
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		addr := env("NHL_STATS_VIM_LISTEN", ":2222")
		err = server.Serve(ctx, addr, os.Getenv("NHL_STATS_VIM_AUTHORIZED_KEYS"), os.Getenv("NHL_STATS_VIM_HOST_KEY"), log)
	} else if len(os.Args) == 1 {
		_, err = tea.NewProgram(ui.NewWithContext(ctx, nhl.NewClient(log)), tea.WithAltScreen()).Run()
	} else {
		err = fmt.Errorf("usage: nhl-stats-vim [serve]")
	}
	if err != nil {
		log.Error("exit", "error", err)
		os.Exit(1)
	}
}
func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
