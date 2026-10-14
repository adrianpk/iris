// Command iris runs the private local conversation service and CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"forge.adrianpk.com/adrianpk/iris/internal/iris"
)

func main() {
	os.Exit(run())
}

func run() int {
	syscall.Umask(0o077)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return iris.NewApp(os.Stdout, os.Stderr).Run(ctx, os.Args[1:])
}
