package iris

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

type historyCLIOptions struct {
	params  historyParams
	project string
	json    bool
	active  bool
	pair    string
	since   string
	until   string
	all     bool
}

func historyFlags(command string) (*flag.FlagSet, *historyCLIOptions) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := &historyCLIOptions{}
	flags.StringVar(&options.project, "project", "", "select registered project")
	flags.BoolVar(&options.params.AllProjects, "all-projects", false, "inspect every project explicitly")
	flags.BoolVar(&options.json, "json", false, "emit JSON")
	flags.StringVar(&options.params.State, "state", "", "pending, active, closed or expired")
	flags.BoolVar(&options.active, "active", false, "select pending and active conversations")
	flags.BoolVar(&options.all, "all", false, "include every lifecycle state (default)")
	flags.StringVar(&options.params.ConversationID, "conversation", "", "select conversation ID")
	flags.IntVar(&options.params.Limit, "limit", 50, "page size, 1-100")
	flags.StringVar(&options.params.Cursor, "cursor", "", "continue the same selection")

	historySearchFlags(flags, options)

	return flags, options
}

func (app *App) historyReference(options historyCLIOptions) (historyParams, error) {
	params := options.params
	if err := options.searchReference(&params); err != nil {
		return params, err
	}

	if options.active && (options.all || params.State != "") {
		return params, &usageError{message: "active, all and state cannot be combined"}
	}

	if options.all && params.State != "" {
		return params, &usageError{message: "all and state cannot be combined"}
	}

	if options.active {
		params.State = "live"
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

func (app *App) runHistory(ctx context.Context, command string, args []string) error {
	flags, options := historyFlags(command)
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, historyHelp(command))
	}

	if command == "show" {
		if err := options.showOperand(flags); err != nil {
			return err
		}
	} else if flags.NArg() != 0 {
		return &usageError{message: "history listing accepts no positional arguments"}
	}

	params, err := app.historyReference(*options)
	if err != nil {
		return err
	}

	reply, err := app.request(ctx, command, params)
	if err != nil {
		return err
	}

	if options.json {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	return app.printHistory(command, reply.Result)
}

func (app *App) printHistory(command string, data json.RawMessage) error {
	if command == "show" {
		return app.printHistoryShow(data)
	}

	var value historyPage
	if err := decodeJSON(data, &value); err != nil {
		return err
	}

	for _, item := range value.Conversations {
		if err := app.printHistoryMatches(item); err != nil {
			return err
		}
	}

	if value.NextCursor != nil {
		_, err := fmt.Fprintln(app.stdout, "Next cursor:", *value.NextCursor)

		return err
	}

	return nil
}

func printHistoryConversation(out io.Writer, value historyConversation) error {
	_, err := fmt.Fprintf(out, "%s %s %s topic=%q created=%s messages=%d\n", value.ID, value.Label, value.State, value.Topic, value.CreatedAt.Format("2006-01-02T15:04:05.000Z"), value.MessageCount)

	return err
}

func (app *App) printHistoryShow(data json.RawMessage) error {
	var value historyShow
	if err := decodeJSON(data, &value); err != nil {
		return err
	}

	if err := printHistoryConversation(app.stdout, value.Conversation); err != nil {
		return err
	}

	for _, value := range value.Messages {
		if _, err := fmt.Fprintf(app.stdout, "%d %s %s: %s\n", value.Sequence, value.CreatedAt.Format("2006-01-02T15:04:05.000Z"), value.SenderAlias, value.Body); err != nil {
			return err
		}
	}

	if value.Truncated {
		_, err := fmt.Fprintln(app.stdout, "Transcript truncated; use iris dump for complete history. Next cursor:", *value.NextCursor)

		return err
	}

	return nil
}

func historySearchFlags(flags *flag.FlagSet, options *historyCLIOptions) {
	flags.StringVar(&options.pair, "pair", "", "two project selectors separated by a comma")
	flags.Int64Var(&options.params.Ordinal, "ordinal", 0, "ordinal within the selected pair")
	flags.StringVar(&options.params.Text, "text", "", "literal case-sensitive substring, at most 512 UTF-8 bytes")
	flags.StringVar(&options.since, "since", "", "earliest creation time, RFC3339")
	flags.StringVar(&options.until, "until", "", "latest creation time, RFC3339")
}

func (options historyCLIOptions) searchReference(params *historyParams) error {
	if options.pair != "" {
		params.Pair = strings.Split(options.pair, ",")
	}

	for _, bound := range []struct {
		text   string
		target **time.Time
	}{{options.since, &params.Since}, {options.until, &params.Until}} {
		if bound.text == "" {
			continue
		}

		value, err := time.Parse(time.RFC3339Nano, bound.text)
		if err != nil {
			return &usageError{message: "dates require RFC3339 timestamps"}
		}

		*bound.target = &value
	}

	return nil
}

func (options *historyCLIOptions) showOperand(flags *flag.FlagSet) error {
	if flags.NArg() == 1 && options.params.ConversationID == "" {
		options.params.ConversationID = flags.Arg(0)

		return nil
	}

	if flags.NArg() == 0 && options.params.Ordinal > 0 && options.pair != "" {
		return nil
	}

	return &usageError{message: "show requires one conversation ID or an explicit pair and ordinal"}
}

func (app *App) printHistoryMatches(item historyConversation) error {
	if err := printHistoryConversation(app.stdout, item); err != nil {
		return err
	}

	for _, match := range item.Matches {
		if _, err := fmt.Fprintf(app.stdout, "  %s sequence=%d %q\n", match.MessageID, match.Sequence, match.Excerpt); err != nil {
			return err
		}
	}

	if item.MatchesTruncated {
		_, err := fmt.Fprintln(app.stdout, "  More matching messages; use show or dump.")

		return err
	}

	return nil
}

func historyHelp(command string) string {
	return "Usage: iris " + command + " [--project <selector>|--all-projects] [--state <state>|--active|--all] [--limit <1-100>] [--cursor <cursor>] [--json] [conversation-id]\nFilters: --conversation <id>, --pair <first,second>, --ordinal <number>, --text <literal>, --since <RFC3339>, --until <RFC3339>.\nShow requires one ID or an explicit pair and ordinal.\n"
}
