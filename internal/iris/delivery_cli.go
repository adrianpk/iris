package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func (app *App) runReceive(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("receive", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	params := receiveParams{}
	selector := flags.String("project", "", "explicit project selector")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	flags.StringVar(&params.ConsumerID, "consumer", "manual", "consumer identity")
	flags.StringVar(&params.Cursor, "cursor", "", "scoped page cursor")
	flags.IntVar(&params.Limit, "limit", 50, "maximum messages in page")
	flags.IntVar(&params.WaitSeconds, "wait", 30, "wait seconds, 0 through 60")

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris receive [--consumer <id>] [--wait <seconds>] [--limit <count>] [--cursor <cursor>] [--project <selector>] [--json] <conversation-id>\nReading does not acknowledge processing.\n")
	}

	if flags.NArg() != 1 {
		return &usageError{message: "receive requires exactly one conversation ID"}
	}

	var err error

	params.projectSelection, err = app.selectReference(*selector)
	if err != nil {
		return err
	}

	params.ConversationID = flags.Arg(0)

	reply, err := app.request(ctx, "receive", params)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var page receivePage
	if err := decodeJSON(reply.Result, &page); err != nil {
		return unavailable(err)
	}

	return app.printReceivePage(page)
}

func (app *App) printReceivePage(page receivePage) error {
	for _, value := range page.Messages {
		if err := app.printMessage(value); err != nil {
			return err
		}
	}

	if len(page.Messages) == 0 {
		_, err := fmt.Fprintf(app.stdout, "No messages for consumer %s; acknowledged through #%d\n", page.ConsumerID, page.AckedSequence)

		return err
	}

	if page.Truncated {
		_, err := fmt.Fprintf(app.stdout, "More messages: --cursor %s\n", *page.NextCursor)

		return err
	}

	return nil
}

func (app *App) runAck(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("ack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	selector := flags.String("project", "", "explicit project selector")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	params := ackParams{}
	flags.StringVar(&params.ConsumerID, "consumer", "manual", "consumer identity")
	flags.StringVar(&params.MessageID, "message-id", "", "next delivered message ID")

	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris ack --message-id <id> [--consumer <id>] [--project <selector>] [--json] <conversation-id>\n")
	}

	if flags.NArg() != 1 || params.MessageID == "" {
		return &usageError{message: "ack requires --message-id and one conversation ID"}
	}

	var err error

	params.projectSelection, err = app.selectReference(*selector)
	if err != nil {
		return err
	}

	params.ConversationID = flags.Arg(0)

	reply, err := app.request(ctx, "ack", params)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var position consumerPosition
	if err := decodeJSON(reply.Result, &position); err != nil {
		return unavailable(err)
	}

	_, err = fmt.Fprintf(app.stdout, "Acknowledged through #%d for consumer %s\n", position.AckedSequence, position.ConsumerID)

	return err
}
