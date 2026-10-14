package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

func (app *App) request(ctx context.Context, operation string, params any) (response, error) {
	cfg, err := app.config()
	if err != nil {
		return response{}, problem("invalid_request", "invalid Iris configuration")
	}

	return requestService(ctx, cfg, operation, params)
}

func (app *App) selectReference(selector string) (projectSelection, error) {
	if selector != "" {
		return projectSelection{Project: selector}, nil
	}

	cwd, err := app.cwd()
	if err != nil {
		return projectSelection{}, problem("project_not_found", "cannot resolve the working directory")
	}

	return projectSelection{CWD: cwd}, nil
}

func (app *App) runProjects(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return app.runProjectList(ctx, nil)
	}

	switch args[0] {
	case "list":
		return app.runProjectList(ctx, args[1:])
	case "rename":
		return app.runProjectRename(ctx, args[1:])
	case "relocate":
		return app.runProjectRelocate(ctx, args[1:])
	case "register":
		return app.runProjectRegister(ctx, args[1:])
	case "--help", "-h":
		_, err := io.WriteString(app.stdout, "Usage: iris projects [list|register|rename|relocate] [options]\n")

		return err
	}

	return &usageError{message: "unknown projects command"}
}

func (app *App) runProjectRegister(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("projects register", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	name := flags.String("name", "", "explicit alias")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris projects register [--name <alias>] [--json] [directory]\n")
	}

	if flags.NArg() > 1 {
		return &usageError{message: "register accepts at most one directory"}
	}

	var (
		path string
		err  error
	)
	if flags.NArg() == 1 {
		path, err = filepath.Abs(flags.Arg(0))
	} else {
		path, err = app.cwd()
	}

	if err != nil {
		return problem("invalid_request", "cannot resolve registration directory")
	}

	reply, err := app.request(ctx, "projects.register", projectRegistration{Path: path, Name: *name})
	if err != nil {
		return err
	}

	return app.printProject(reply, *jsonOutput)
}

func (app *App) printProject(reply response, jsonOutput bool) error {
	if jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var value project
	if err := decodeJSON(reply.Result, &value); err != nil {
		return unavailable(err)
	}

	path := "unregistered"
	if value.Path != nil {
		path = *value.Path
	}

	_, err := fmt.Fprintf(app.stdout, "%s %s %s\n", value.ID, value.Alias, path)

	return err
}

func (app *App) runProjectList(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("projects list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	limit := flags.Int("limit", 50, "page size")
	cursor := flags.String("cursor", "", "next page cursor")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris projects list [--limit <count>] [--cursor <id>] [--json]\n")
	}

	if flags.NArg() != 0 {
		return &usageError{message: "projects list accepts no positional arguments"}
	}

	reply, err := app.request(ctx, "projects.list", projectListParams{Limit: *limit, Cursor: *cursor})
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var page projectPage
	if err := decodeJSON(reply.Result, &page); err != nil {
		return unavailable(err)
	}

	for _, value := range page.Projects {
		if err := app.printProject(success(value), false); err != nil {
			return err
		}
	}

	if page.Truncated {
		_, err = fmt.Fprintf(app.stdout, "More projects: --cursor %s\n", *page.NextCursor)
	}

	return err
}
