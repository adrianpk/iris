package iris

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cliDelivery(t *testing.T, cfg config, path, input string, args ...string) (int, []byte, []byte) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	app := NewApp(&stdout, &stderr)
	app.config = func() (config, error) { return cfg, nil }
	app.cwd = func() (string, error) { return path, nil }
	app.stdin = strings.NewReader(input)
	code := app.Run(t.Context(), args)

	return code, stdout.Bytes(), stderr.Bytes()
}

// CLI text/file/stdin preserve UTF-8 and multiline content across the actual Unix boundary.
func TestDeliveryCLIInputs(t *testing.T) {
	f := newDeliveryFixture(t)
	cfg, _ := testService(t, f.store.handle)
	path := filepath.Join(t.TempDir(), "content.txt")

	body := "A multiline request\nwith Unicode: café.\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, input string
		args        []string
	}{
		{"text", "", []string{"--message", body}},
		{"file", "", []string{"--file", path}},
		{"stdin", body, nil},
		{"body boundary", strings.Repeat("x", maxMessageBodyBytes), nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"send", "--json"}, test.args...)
			args = append(args, f.conversation.ID)

			expected := body
			if test.name == "body boundary" {
				expected = test.input
			}

			assertSendCLIContent(t, cfg, *f.alpha.Path, test.input, expected, args...)
		})
	}
}

// Invalid or oversized content leaves stdout empty and does not allocate a stored message.
func TestDeliveryCLIErrors(t *testing.T) {
	f := newDeliveryFixture(t)
	cfg, _ := testService(t, f.store.handle)

	cases := []struct {
		name, input string
		args        []string
		exit        int
	}{
		{"empty stdin", "", nil, 1},
		{"empty text", "", []string{"--message", ""}, 1},
		{"ambiguous", "", []string{"--message", "body", "--file", "file.txt"}, 2},
		{"oversized", strings.Repeat("x", maxMessageBodyBytes+1), nil, 1},
		{"invalid UTF8", string([]byte{0xff}), nil, 1},
		{"forged system", "", []string{"--message", "body", "--kind", "system"}, 1},
		{"empty key", "", []string{"--message", "body", "--idempotency-key", ""}, 1},
		{"large correlation", "", []string{"--message", "body", "--correlation-id", strings.Repeat("x", 129)}, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"send", "--json"}, test.args...)
			args = append(args, f.conversation.ID)

			code, stdout, stderr := cliDelivery(t, cfg, *f.alpha.Path, test.input, args...)
			if code != test.exit || len(stdout) != 0 {
				t.Fatalf("exit=%d stdout=%s", code, stdout)
			}

			var reply response
			if err := decodeJSON(stderr, &reply); err != nil || reply.Error == nil {
				t.Fatalf("stderr=%s error=%v", stderr, err)
			}
		})
	}

	assertHandshakeRows(t, f.store, f.conversation.ID, "active", 2, 0)
}

// Human and JSON receive expose durable IDs, and acknowledgment advances only through explicit CLI action.
func TestDeliveryCLIReceiveAck(t *testing.T) {
	f := newDeliveryFixture(t)
	cfg, _ := testService(t, f.store.handle)
	sent := mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "Readable response"))

	assertHumanReceive(t, cfg, f, sent)

	code, stdout, stderr := cliDelivery(t, cfg, *f.beta.Path, "", "receive", "--wait", "0", "--json", f.conversation.ID)
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("JSON receive exit=%d stderr=%s", code, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		t.Fatal(err)
	}

	var page receivePage
	if err := decodeJSON(reply.Result, &page); err != nil {
		t.Fatal(err)
	}

	if page.AckedSequence != 0 || len(page.Messages) != 3 {
		t.Fatalf("receive acknowledged work: %+v", page)
	}

	assertCLIAck(t, cfg, f, page.Messages[0])
}

func assertCLIAck(t *testing.T, cfg config, f deliveryFixture, value message) {
	t.Helper()

	code, stdout, stderr := cliDelivery(t, cfg, *f.beta.Path, "", "ack", "--message-id", value.ID, "--json", f.conversation.ID)
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("ack exit=%d stderr=%s", code, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		t.Fatal(err)
	}

	var position consumerPosition
	if err := decodeJSON(reply.Result, &position); err != nil || position.AckedSequence != 1 {
		t.Fatalf("ack=%s error=%v", stdout, err)
	}
}

func assertSendCLIContent(t *testing.T, cfg config, path, input, expected string, args ...string) {
	t.Helper()

	code, stdout, stderr := cliDelivery(t, cfg, path, input, args...)
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("send exit=%d stderr=%s", code, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		t.Fatal(err)
	}

	var sent message
	if err := decodeJSON(reply.Result, &sent); err != nil {
		t.Fatal(err)
	}

	if sent.Body != expected || !reply.OK {
		t.Fatalf("content changed: bytes=%d", len(sent.Body))
	}
}

func assertHumanReceive(t *testing.T, cfg config, f deliveryFixture, sent message) {
	t.Helper()

	code, stdout, stderr := cliDelivery(t, cfg, *f.beta.Path, "", "receive", "--wait", "0", f.conversation.ID)
	if code != 0 || len(stderr) != 0 || !strings.Contains(string(stdout), sent.ID) || !strings.Contains(string(stdout), sent.Body) {
		t.Fatalf("human receive exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
