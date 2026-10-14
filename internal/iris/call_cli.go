package iris

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

type callOptions struct {
	project *string
	to      *string
	topic   *string
	key     *string
	body    *string
	file    *string
	json    *bool
}

func addCallFlags(flags *flag.FlagSet) callOptions {
	return callOptions{
		project: flags.String("project", "", "explicit project selector"),
		to:      flags.String("to", "", "recipient alias, ID or directory"),
		topic:   flags.String("topic", "", "conversation topic"),
		key:     flags.String("idempotency-key", "", "stable request key for retries"),
		body:    flags.String("message", "", "initial message text"),
		file:    flags.String("file", "", "read message file"),
		json:    flags.Bool("json", false, "emit JSON"),
	}
}

func (app *App) runCall(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("call", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := addCallFlags(flags)
	if err := flags.Parse(args); err != nil {
		return app.flagError(err, "Usage: iris call --to <project> --topic <topic> [--message <text>|--file <path>] [--project <selector>] [--idempotency-key <key>] [--json]\nWithout --message or --file, read initial content from stdin.\n")
	}

	if flags.NArg() != 0 || *options.to == "" || *options.topic == "" {
		return &usageError{message: "call requires --to, --topic, initial content and no positional arguments"}
	}

	params, err := app.callArguments(flags, options)
	if err != nil {
		return err
	}

	reply, err := app.request(ctx, "call", params)
	if err != nil {
		return err
	}

	return app.printConversation(reply, *options.json)
}

func (app *App) callArguments(flags *flag.FlagSet, options callOptions) (callParams, error) {
	selection, err := app.selectReference(*options.project)
	if err != nil {
		return callParams{}, err
	}

	body, err := app.messageContent(flags, *options.body, *options.file)
	if err != nil {
		return callParams{}, err
	}

	key := *options.key
	supplied := false

	flags.Visit(func(value *flag.Flag) {
		if value.Name == "idempotency-key" {
			supplied = true
		}
	})

	if !supplied {
		key, err = newID()
		if err != nil {
			return callParams{}, err
		}
	}

	return callParams{projectSelection: selection, To: *options.to, Topic: *options.topic, Body: body, RequestKey: key}, nil
}

func (app *App) messageContent(flags *flag.FlagSet, body, path string) (string, error) {
	messageSet, fileSet := false, false

	flags.Visit(func(value *flag.Flag) {
		switch value.Name {
		case "message":
			messageSet = true
		case "file":
			fileSet = true
		}
	})

	if messageSet && fileSet {
		return "", &usageError{message: "--message and --file are mutually exclusive"}
	}

	if messageSet {
		return checkedMessageContent(body)
	}

	reader := app.stdin

	if fileSet {
		return messageContentFile(path)
	}

	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice != 0 {
			return "", &usageError{message: "content requires --message, --file or piped stdin"}
		}
	}

	return readMessageContent(reader)
}

func readMessageContent(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxMessageBodyBytes+1))
	if err != nil {
		return "", problem("invalid_request", "cannot read message")
	}

	return checkedMessageContent(string(data))
}

func checkedMessageContent(body string) (string, error) {
	if !validText(body, maxMessageBodyBytes) {
		return "", problem("invalid_request", "message must be nonempty UTF-8 within the body limit")
	}

	return body, nil
}

func (app *App) printConversation(reply response, jsonOutput bool) error {
	if jsonOutput {
		return writeFrame(app.stdout, reply, maxResponseBytes)
	}

	var value conversation
	if err := decodeJSON(reply.Result, &value); err != nil {
		return unavailable(err)
	}

	_, err := fmt.Fprintf(app.stdout, "%s\nID: %s\nState: %s\nInvitation: %s\nTopic: %s\nInitial message from %s:\n%s\n", value.Label, value.ID, value.State, value.InvitationCode, value.Topic, value.InitialMessage.SenderAlias, value.InitialMessage.Body)

	return err
}

func messageContentFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", problem("invalid_request", "cannot open message file")
	}

	content, readErr := readMessageContent(file)
	closeErr := file.Close()

	if readErr != nil {
		return "", readErr
	}

	if closeErr != nil {
		return "", problem("invalid_request", "cannot close message file")
	}

	return content, nil
}
