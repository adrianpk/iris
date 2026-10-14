package iris

import (
	"context"
	"flag"
	"io"
	"path/filepath"
)

func (app *App) runProjectRename(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("projects rename", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	name := flags.String("name", "", "new alias")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris projects rename --name <alias> [--json] <project-id>\n")
	}

	if flags.NArg() != 1 || *name == "" {
		return &usageError{message: "rename requires --name and one project ID"}
	}

	reply, err := app.request(ctx, "projects.rename", projectRename{ProjectID: flags.Arg(0), Name: *name})
	if err != nil {
		return err
	}

	return app.printProject(reply, *jsonOutput)
}

func (app *App) runProjectRelocate(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("projects relocate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	path := flags.String("path", "", "new directory")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris projects relocate --path <directory> [--json] <project-id>\n")
	}

	if flags.NArg() != 1 || *path == "" {
		return &usageError{message: "relocate requires --path and one project ID"}
	}

	absolute, err := filepath.Abs(*path)
	if err != nil {
		return problem("invalid_request", "cannot resolve relocation directory")
	}

	reply, err := app.request(ctx, "projects.relocate", projectRelocation{ProjectID: flags.Arg(0), Path: absolute})
	if err != nil {
		return err
	}

	return app.printProject(reply, *jsonOutput)
}
