package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func (app *App) runHandshake(ctx context.Context, operation string, args []string) error {
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	selector := flags.String("project", "", "explicit project selector")
	jsonOutput := flags.Bool("json", false, "emit JSON")

	operand := "conversation-id"
	if operation == "join" {
		operand = "invitation-code"
	}

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, fmt.Sprintf("Usage: iris %s [--project <selector>] [--json] <%s>\n", operation, operand))
	}

	if flags.NArg() != 1 {
		return &usageError{message: operation + " requires exactly one " + operand}
	}

	selection, err := app.selectReference(*selector)
	if err != nil {
		return err
	}

	var params any = conversationParams{projectSelection: selection, ConversationID: flags.Arg(0)}
	if operation == "join" {
		params = joinParams{projectSelection: selection, Code: flags.Arg(0)}
	}

	reply, err := app.request(ctx, operation, params)
	if err != nil {
		return err
	}

	return app.printConversation(reply, *jsonOutput)
}
