// Package iris implements the local Iris service, CLI, and persistence boundary.
package iris

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
)

// App owns command dispatch and process-level dependency assembly.
type App struct {
	stdout io.Writer
	stderr io.Writer
	config func() (config, error)
	cwd    func() (string, error)
	stdin  io.Reader
}

// NewApp stores output streams without resolving paths or performing IO.
func NewApp(stdout, stderr io.Writer) *App {
	return &App{stdout: stdout, stderr: stderr, config: defaultConfig, cwd: os.Getwd, stdin: os.Stdin}
}

// Run executes one command and returns its process exit code.
func (app *App) Run(ctx context.Context, args []string) int {
	if err := app.run(ctx, args); err != nil {
		if writeErr := app.reportError(err, slices.Contains(args, "--json") || slices.Contains(args, "--json=true")); writeErr != nil {
			return 1
		}

		return exitCode(err)
	}

	return 0
}

func (app *App) run(ctx context.Context, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(app.stdout, rootHelp)

		return err
	}

	switch args[0] {
	case "serve":
		return app.runServe(ctx, args[1:])
	case "projects":
		return app.runProjects(ctx, args[1:])
	case "stats":
		return app.runStats(ctx, args[1:])
	}

	return &usageError{message: fmt.Sprintf("unknown command %q; run iris --help", args[0])}
}

type usageError struct {
	message string
}

func (err *usageError) Error() string {
	return err.message
}

func exitCode(err error) int {
	if errors.Is(err, context.Canceled) {
		return 130
	}

	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}

	return 1
}
