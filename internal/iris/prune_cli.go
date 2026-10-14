package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"
)

type pruneCLIOptions struct {
	params  pruneParams
	project string
	before  string
	confirm string
	dryRun  bool
	json    bool
}

func (app *App) runPrune(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := pruneCLIOptions{}
	flags.StringVar(&options.project, "project", "", "select registered project")
	flags.BoolVar(&options.params.AllProjects, "all-projects", false, "inspect all projects explicitly")
	flags.StringVar(&options.params.ConversationID, "conversation", "", "select one conversation ID")
	flags.StringVar(&options.params.State, "state", "", "closed or expired")
	flags.StringVar(&options.before, "before", "", "latest terminal time, RFC3339")
	flags.StringVar(&options.confirm, "confirm", "", "confirm the exact preview token")
	flags.BoolVar(&options.dryRun, "dry-run", false, "preview without deleting history")
	flags.BoolVar(&options.json, "json", false, "emit JSON")

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris prune [--project <selector>|--all-projects] [--conversation <id>] [--state closed|expired] [--before <RFC3339>] [--dry-run] [--json]\n       iris prune --confirm <token> [--json]\nBare prune previews; only exact confirmation deletes history.\n")
	}

	if flags.NArg() != 0 {
		return &usageError{message: "prune accepts no positional arguments"}
	}

	if options.confirm != "" {
		return app.runPruneConfirmation(ctx, options)
	}

	return app.runPrunePreview(ctx, options)
}

func (app *App) runPruneConfirmation(ctx context.Context, options pruneCLIOptions) error {
	if options.hasFilters() || options.dryRun {
		return &usageError{message: "confirmation token cannot be combined with selection or dry-run flags"}
	}

	reply, err := app.request(ctx, "prune.confirm", pruneConfirmation{Token: options.confirm})
	if err != nil {
		return err
	}

	if options.json {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var result pruneResult
	if err := decodeJSON(reply.Result, &result); err != nil {
		return err
	}

	_, err = fmt.Fprintf(app.stdout, "Pruned %d conversations, %d messages, %d body bytes.\n", len(result.ConversationIDs), result.Messages, result.BodyBytes)

	return err
}

func (options pruneCLIOptions) hasFilters() bool {
	return options.project != "" || options.params.AllProjects || options.params.ConversationID != "" || options.params.State != "" || options.before != ""
}

func (app *App) runPrunePreview(ctx context.Context, options pruneCLIOptions) error {
	params, err := app.pruneReference(options)
	if err != nil {
		return err
	}

	reply, err := app.request(ctx, "prune.preview", params)
	if err != nil {
		return err
	}

	if options.json {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var value prunePreview
	if err := decodeJSON(reply.Result, &value); err != nil {
		return err
	}

	return app.printPrunePreview(value)
}

func (app *App) pruneReference(options pruneCLIOptions) (pruneParams, error) {
	params := options.params
	if options.before != "" {
		value, err := time.Parse(time.RFC3339Nano, options.before)
		if err != nil {
			return params, &usageError{message: "before requires an RFC3339 timestamp"}
		}

		params.Before = &value
	}

	if params.AllProjects {
		if options.project != "" {
			return params, &usageError{message: "select a project or all-projects scope"}
		}

		return params, nil
	}

	ref, err := app.selectReference(options.project)
	params.projectSelection = ref

	return params, err
}

func (app *App) printPrunePreview(value prunePreview) error {
	if _, err := fmt.Fprintf(app.stdout, "Preview: %d eligible conversations, %d messages, %d body bytes.\n", len(value.Candidates), value.Messages, value.BodyBytes); err != nil {
		return err
	}

	for _, candidate := range value.Candidates {
		if _, err := fmt.Fprintf(app.stdout, "%s %s terminal=%s messages=%d bytes=%d\n", candidate.ConversationID, candidate.State, candidate.TerminalAt.Format(time.RFC3339Nano), candidate.MessageCount, candidate.BodyBytes); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(app.stdout, "Confirmation token: %s\nExpires: %s\n", value.Token, value.ExpiresAt.Format(time.RFC3339Nano))

	return err
}
