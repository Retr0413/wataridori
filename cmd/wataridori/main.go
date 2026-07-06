package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Retr0413/wataridori/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := cli.Execute(ctx)
	if err == nil {
		return
	}
	if errors.Is(err, cli.ErrDrift) {
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
