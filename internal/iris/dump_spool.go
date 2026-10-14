package iris

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func spoolDump(ctx context.Context, cfg config, snapshot dumpSnapshot, options dumpCLIOptions, out io.Writer) error {
	if err := writeDumpHeader(out, snapshot, options); err != nil {
		return err
	}

	params := dumpLeaseParams{LeaseID: snapshot.LeaseID}
	sequence := int64(0)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		page, err := serviceValue[dumpPage](ctx, cfg, "dump.page", params)
		if err != nil {
			return err
		}

		sequence, err = writeDumpMessages(out, page.Messages, sequence, snapshot, options.format)
		if err != nil {
			return err
		}

		if !page.Truncated {
			break
		}

		if page.NextCursor == nil || len(page.Messages) == 0 {
			return problem("internal_error", "export page made no progress")
		}

		params.Cursor = *page.NextCursor
	}

	if sequence != snapshot.HighestSequence {
		return problem("internal_error", "export snapshot is incomplete")
	}

	return writeDumpFooter(out, options)
}

func writeDumpHeader(out io.Writer, snapshot dumpSnapshot, options dumpCLIOptions) error {
	if options.format == "text" {
		return writeDumpTextHeader(out, snapshot)
	}

	if options.json {
		if _, err := io.WriteString(out, `{"protocol":"iris/v1","schema_version":1,"ok":true,"result":`); err != nil {
			return err
		}
	}

	metadata, err := json.Marshal(struct {
		SchemaVersion int                 `json:"schema_version"`
		Conversation  historyConversation `json:"conversation"`
		Participants  []participant       `json:"participants"`
	}{1, snapshot.Conversation, snapshot.Participants})
	if err != nil {
		return err
	}

	if _, err := out.Write(metadata[:len(metadata)-1]); err != nil {
		return err
	}

	_, err = io.WriteString(out, `,"messages":[`)

	return err
}

func writeDumpTextHeader(out io.Writer, snapshot dumpSnapshot) error {
	value := snapshot.Conversation
	if _, err := fmt.Fprintf(out, "Iris transcript: %s\nConversation: %s\nTopic: %s\nState: %s\nCreated: %s\nLast activity: %s\n", value.Label, value.ID, value.Topic, value.State, value.CreatedAt.Format(time.RFC3339Nano), value.LastActivityAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}

	if value.TerminalAt != nil {
		if _, err := fmt.Fprintf(out, "Terminal: %s\n", value.TerminalAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}

	for _, p := range snapshot.Participants {
		joined := "not joined"
		if p.JoinedAt != nil {
			joined = p.JoinedAt.Format(time.RFC3339Nano)
		}

		if _, err := fmt.Fprintf(out, "Participant: %s %s joined=%s\n", p.ProjectID, p.Alias, joined); err != nil {
			return err
		}
	}

	_, err := io.WriteString(out, "\n")

	return err
}

func writeDumpMessages(out io.Writer, messages []message, after int64, snapshot dumpSnapshot, format string) (int64, error) {
	for _, value := range messages {
		if value.ConversationID != snapshot.Conversation.ID || value.Sequence != after+1 || value.Sequence > snapshot.HighestSequence {
			return after, problem("internal_error", "export message order does not match its snapshot")
		}

		if err := writeDumpMessage(out, value, after, format); err != nil {
			return after, err
		}

		after = value.Sequence
	}

	return after, nil
}

func writeDumpMessage(out io.Writer, value message, after int64, format string) error {
	if format == "text" {
		if _, err := fmt.Fprintf(out, "[%d] %s %s (%s) message=%s\n", value.Sequence, value.CreatedAt.Format(time.RFC3339Nano), value.SenderAlias, value.Kind, value.ID); err != nil {
			return err
		}

		_, err := fmt.Fprintf(out, "%s\n\n", value.Body)

		return err
	}

	if after > 0 {
		if _, err := io.WriteString(out, ","); err != nil {
			return err
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	_, err = out.Write(encoded)

	return err
}

func writeDumpFooter(out io.Writer, options dumpCLIOptions) error {
	if options.format == "text" {
		return nil
	}

	suffix := "]}\n"
	if options.json {
		suffix = "]}}\n"
	}

	_, err := io.WriteString(out, suffix)

	return err
}
