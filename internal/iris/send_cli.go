package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

type sendOptions struct {
	params                                 sendParams
	project, file, key, reply, correlation string
	json                                   bool
}

func addSendFlags(flags *flag.FlagSet) *sendOptions {
	options := &sendOptions{}
	flags.StringVar(&options.project, "project", "", "explicit project selector")
	flags.StringVar(&options.file, "file", "", "read message file")
	flags.StringVar(&options.params.Body, "message", "", "message text")
	flags.StringVar(&options.params.Kind, "kind", "message", "message or status")
	flags.StringVar(&options.key, "idempotency-key", "", "stable retry key")
	flags.StringVar(&options.reply, "reply-to", "", "message ID in this conversation")
	flags.StringVar(&options.correlation, "correlation-id", "", "caller correlation ID")
	flags.BoolVar(&options.json, "json", false, "emit JSON")
	flags.Func("to", "recipient project IDs, comma separated; defaults to the other participant", func(value string) error {
		options.params.Recipients = append(options.params.Recipients, strings.Split(value, ",")...)

		return nil
	})

	return options
}

func (app *App) runSend(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := addSendFlags(flags)
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris send [--message <text>|--file <path>] [--kind message|status] [--to <project-ids>] [--reply-to <message-id>] [--correlation-id <id>] [--idempotency-key <key>] [--project <selector>] [--json] <conversation-id>\nWithout --message or --file, read content from stdin.\n")
	}

	if flags.NArg() != 1 {
		return &usageError{message: "send requires exactly one conversation ID"}
	}

	params, err := app.sendArguments(flags, options)
	if err != nil {
		return err
	}

	reply, err := app.request(ctx, "send", params)
	if err != nil {
		return err
	}

	if options.json {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var value message
	if err := decodeJSON(reply.Result, &value); err != nil {
		return unavailable(err)
	}

	return app.printMessage(value)
}

func (app *App) sendArguments(flags *flag.FlagSet, options *sendOptions) (sendParams, error) {
	params := options.params

	var err error

	params.projectSelection, err = app.selectReference(options.project)
	if err != nil {
		return params, err
	}

	params.Body, err = app.messageContent(flags, params.Body, options.file)
	if err != nil {
		return params, err
	}

	params.ConversationID = flags.Arg(0)
	params.ReplyTo = optionalFlag(flags, "reply-to", options.reply)
	params.CorrelationID = optionalFlag(flags, "correlation-id", options.correlation)
	params.IdempotencyKey = optionalFlag(flags, "idempotency-key", options.key)

	return params, nil
}

func optionalFlag(flags *flag.FlagSet, name, value string) *string {
	var result *string

	flags.Visit(func(option *flag.Flag) {
		if option.Name == name {
			result = &value
		}
	})

	return result
}

func (app *App) printMessage(value message) error {
	_, err := fmt.Fprintf(app.stdout, "#%d %s %s %s (%s)\n%s\n", value.Sequence, value.ID, value.CreatedAt.Format(time.RFC3339Nano), value.SenderAlias, value.Kind, value.Body)

	return err
}
