package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
)

type bridgeCLIOptions struct {
	params  bridgeClaimParams
	project string
	json    bool
}

func addBridgeFlags(flags *flag.FlagSet) *bridgeCLIOptions {
	options := &bridgeCLIOptions{params: bridgeClaimParams{Options: defaultBridgeOptions()}}
	flags.StringVar(&options.project, "project", "", "explicit project selector")
	flags.BoolVar(&options.json, "json", false, "emit JSON on termination")
	flags.StringVar(&options.params.Options.Model, "model", "", "Codex model; defaults to its configured model")
	flags.StringVar(&options.params.Options.Sandbox, "sandbox", "read-only", "read-only or workspace-write")
	flags.StringVar(&options.params.Options.Approvals, "approvals", "deny", "deny or interactive")
	flags.IntVar(&options.params.Options.TurnSeconds, "turn-seconds", 120, "single-turn deadline")
	flags.IntVar(&options.params.Options.MaxTurns, "max-turns", 20, "maximum invoked turns")
	flags.IntVar(&options.params.Options.TotalSeconds, "total-seconds", 600, "total execution lifetime")
	flags.Int64Var(&options.params.Options.MaxTokens, "max-tokens", 100000, "reported-token budget")

	return options
}

func (app *App) runBridge(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(app.stdout, "Usage: iris bridge start|recover|status|stop [options] <conversation-id>\nStart runs a separate foreground worker; status and stop use the selected project.\n")

		return err
	}

	if args[0] == "recover" {
		return app.runBridgeRecover(ctx, args[1:])
	}

	if args[0] == "start" {
		return app.runBridgeStart(ctx, args[1:])
	}

	if args[0] == "status" || args[0] == "stop" {
		return app.runBridgeControl(ctx, args[0], args[1:])
	}

	return &usageError{message: "unknown bridge command"}
}

func (app *App) runBridgeStart(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("bridge start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := addBridgeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris bridge start [--project <selector>] [--sandbox read-only|workspace-write] [--approvals deny|interactive] [--model <id>] [--turn-seconds <n>] [--max-turns <n>] [--total-seconds <n>] [--max-tokens <n>] [--json] <conversation-id>\n")
	}

	if flags.NArg() != 1 {
		return &usageError{message: "bridge start requires one conversation ID"}
	}

	if _, err := app.bridgeTerminal(options.params.Options); err != nil {
		return err
	}

	selection, err := app.selectReference(options.project)
	if err != nil {
		return err
	}

	options.params.projectSelection, options.params.ConversationID = selection, flags.Arg(0)

	cfg, err := app.config()
	if err != nil {
		return problem("invalid_request", "invalid Iris configuration")
	}

	claim, err := serviceValue[bridgeClaim](ctx, cfg, "bridge.claim", options.params)
	if err != nil {
		return err
	}

	value, err := app.runBridgeOwner(ctx, cfg, claim)
	if err != nil {
		return err
	}

	return app.printBridge(value, options.json)
}

func (app *App) runBridgeControl(ctx context.Context, operation string, args []string) error {
	flags := flag.NewFlagSet("bridge "+operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	selector := flags.String("project", "", "explicit project selector")

	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris bridge "+operation+" [--project <selector>] [--json] <conversation-id>\n")
	}

	if flags.NArg() != 1 {
		return &usageError{message: "bridge " + operation + " requires one conversation ID"}
	}

	selection, err := app.selectReference(*selector)
	if err != nil {
		return err
	}

	reply, err := app.request(ctx, "bridge."+operation, conversationParams{projectSelection: selection, ConversationID: flags.Arg(0)})
	if err != nil {
		return err
	}

	var value bridgeBinding
	if err := decodeJSON(reply.Result, &value); err != nil {
		return err
	}

	return app.printBridge(value, *jsonOutput)
}

func (app *App) printBridge(value bridgeBinding, jsonOutput bool) error {
	if jsonOutput {
		return writeFrame(app.stdout, success(value), maxResponseBytes)
	}

	_, err := fmt.Fprintf(app.stdout, "Bridge %s: %s; project %s; conversation %s; turns %d/%d\n", value.ID, value.State, value.ProjectID, value.ConversationID, value.TurnsUsed, value.Options.MaxTurns)
	if err != nil {
		return err
	}

	return app.printBridgeRecovery(value)
}

func (app *App) runBridgeRecover(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("bridge recover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	project := flags.String("project", "", "explicit project selector")

	jsonOutput := flags.Bool("json", false, "emit JSON on termination")
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris bridge recover [--project <selector>] [--json] <conversation-id>\nInspect only the recorded owned turn; completed results can resume without another invocation.\n")
	}

	if flags.NArg() != 1 {
		return &usageError{message: "bridge recover requires one conversation ID"}
	}

	selection, err := app.selectReference(*project)
	if err != nil {
		return err
	}

	cfg, err := app.config()
	if err != nil {
		return problem("invalid_request", "invalid Iris configuration")
	}

	claim, err := serviceValue[bridgeClaim](ctx, cfg, "bridge.recover", conversationParams{projectSelection: selection, ConversationID: flags.Arg(0)})
	if err != nil {
		return err
	}

	value, err := app.runBridgeOwner(ctx, cfg, claim)
	if err != nil {
		return err
	}

	return app.printBridge(value, *jsonOutput)
}

func (app *App) printBridgeRecovery(value bridgeBinding) error {
	if value.ErrorCode != nil {
		if _, err := fmt.Fprintln(app.stdout, "Error:", *value.ErrorCode); err != nil {
			return err
		}
	}

	if value.RecoveryHint != nil {
		_, err := fmt.Fprintln(app.stdout, *value.RecoveryHint)

		return err
	}

	return nil
}
