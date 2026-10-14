package iris

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"time"
)

type dumpCLIOptions struct {
	historyCLIOptions
	format string
}

func (app *App) runDump(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("dump", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := dumpCLIOptions{}
	flags.StringVar(&options.project, "project", "", "select registered project")
	flags.BoolVar(&options.params.AllProjects, "all-projects", false, "inspect all projects explicitly")
	flags.StringVar(&options.pair, "pair", "", "two project selectors separated by a comma")
	flags.Int64Var(&options.params.Ordinal, "ordinal", 0, "ordinal within the selected pair")
	flags.StringVar(&options.format, "format", "text", "text or json")
	flags.BoolVar(&options.json, "json", false, "emit a JSON result envelope")

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris dump [--project <selector>|--all-projects] [--format text|json] [--json] <id>\n       iris dump [scope] --pair <first,second> --ordinal <number> [--format text|json]\n")
	}

	if err := options.showOperand(flags); err != nil {
		return err
	}

	if options.format != "text" && options.format != "json" {
		return &usageError{message: "dump format must be text or json"}
	}

	if options.json {
		options.format = "json"
	}

	params, err := app.historyReference(options.historyCLIOptions)
	if err != nil {
		return err
	}

	cfg, err := app.config()
	if err != nil {
		return err
	}

	snapshot, err := serviceValue[dumpSnapshot](ctx, cfg, "dump.open", params)
	if err != nil {
		return err
	}

	return app.exportDump(ctx, cfg, snapshot, options)
}

func (app *App) exportDump(ctx context.Context, cfg config, snapshot dumpSnapshot, options dumpCLIOptions) (resultErr error) {
	released := false
	defer func() {
		if released {
			return
		}

		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()

		_, err := serviceValue[dumpClosed](cleanup, cfg, "dump.close", dumpLeaseParams{LeaseID: snapshot.LeaseID})
		resultErr = errors.Join(resultErr, err)
	}()

	file, err := os.CreateTemp(cfg.runtimeRoot, "dump-*.tmp")
	if err != nil {
		return problem("storage_unavailable", "cannot create private export spool")
	}

	defer func() { resultErr = errors.Join(resultErr, file.Close(), os.Remove(file.Name())) }()

	writer := &exportSpool{writer: file, remaining: maxExportSpoolBytes}
	if err := spoolDump(ctx, cfg, snapshot, options, writer); err != nil {
		return err
	}

	if _, err := serviceValue[dumpClosed](ctx, cfg, "dump.close", dumpLeaseParams{LeaseID: snapshot.LeaseID}); err != nil {
		return err
	}

	released = true

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	_, err = io.Copy(app.stdout, &exportReader{ctx: ctx, reader: file})

	return err
}

type exportSpool struct {
	writer    io.Writer
	remaining int64
}

func (writer *exportSpool) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		return 0, problem("limit_exceeded", "export exceeds the 128 MiB spool limit")
	}

	n, err := writer.writer.Write(data)
	writer.remaining -= int64(n)

	return n, err
}

type exportReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *exportReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}

	return reader.reader.Read(data)
}
