package iris

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
)

const rootHelp = `Usage: iris <command> [options]

Commands:
  serve                       Run the private local service
  stats --all-projects [--json] Inspect service readiness and storage counts
  help                        Show command help

The service uses XDG data/config/runtime roots. No network listener is opened.
`

func (app *App) runServe(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris serve\n")
	}

	if flags.NArg() != 0 {
		return &usageError{message: "serve accepts no positional arguments"}
	}

	cfg, err := app.config()
	if err != nil {
		return &serviceError{Code: "invalid_request", Message: "invalid Iris configuration", cause: err}
	}

	return serveConfigured(ctx, cfg)
}

func serveConfigured(ctx context.Context, cfg config) (resultErr error) {
	lock, err := acquireRuntime(cfg)
	if err != nil {
		return unavailable(err)
	}
	defer func() { resultErr = errors.Join(resultErr, lock.Close()) }()

	s, err := openStore(ctx, cfg)
	if err != nil {
		return storageFailure(err)
	}

	defer func() { resultErr = errors.Join(resultErr, s.close()) }()

	listener, err := listenUnix(cfg.socket)
	if err != nil {
		return unavailable(err)
	}

	defer func() {
		if err := listener.Close(); !errors.Is(err, net.ErrClosed) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	service := server{config: cfg, handle: s.handle}

	return service.serve(ctx, listener)
}

func (app *App) flagError(err error, help string) error {
	if errors.Is(err, flag.ErrHelp) {
		_, writeErr := io.WriteString(app.stdout, help)

		return writeErr
	}

	return &usageError{message: err.Error()}
}

func (app *App) runStats(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("stats", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	all := flags.Bool("all-projects", false, "inspect all projects")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris stats --all-projects [--json]\n")
	}

	if flags.NArg() != 0 || !*all {
		return &usageError{message: "stats requires --all-projects and no positional arguments"}
	}

	cfg, err := app.config()
	if err != nil {
		return &serviceError{Code: "invalid_request", Message: "invalid Iris configuration", cause: err}
	}

	reply, err := callService(ctx, cfg, request{Protocol: protocolVersion, Operation: "stats", Params: []byte(`{"all_projects":true}`)})
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var result statistics
	if err := decodeJSON(reply.Result, &result); err != nil {
		return unavailable(err)
	}

	_, err = fmt.Fprintf(app.stdout, "Iris ready; schema %d; %d projects; %d conversations; %d messages\n", result.StorageSchemaVersion, result.Projects, result.Conversations, result.Messages)

	return err
}

func (app *App) reportError(err error, jsonOutput bool) error {
	if !jsonOutput {
		_, writeErr := fmt.Fprintln(app.stderr, err)

		return writeErr
	}

	var service *serviceError
	if !errors.As(err, &service) {
		service = &serviceError{Code: "invalid_request", Message: err.Error()}
	}

	return writeFrame(app.stderr, failure(service.Code, service.Message, service.Retryable), maxResponseBytes)
}

func storageFailure(err error) *serviceError {
	if errors.Is(err, errSchemaMismatch) {
		return &serviceError{Code: "schema_mismatch", Message: "database schema does not match this executable", cause: err}
	}

	return &serviceError{Code: "storage_unavailable", Message: "cannot initialize private SQLite storage", cause: err}
}
