package iris

import (
	"bytes"
	"strings"
	"testing"
)

// Command help returns before configuration and service access.
func TestCommandHelp(t *testing.T) {
	for _, command := range []string{"serve", "stats"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			app := NewApp(&stdout, &stderr)

			app.config = func() (config, error) { t.Fatal("help loaded configuration"); return config{}, nil }
			if code := app.Run(t.Context(), []string{command, "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

// JSON output is one complete envelope, while usage failures keep stdout empty.
func TestStatsOutput(t *testing.T) {
	cfg, _ := testService(t, nil)

	var stdout, stderr bytes.Buffer

	app := NewApp(&stdout, &stderr)

	app.config = func() (config, error) { return cfg, nil }
	if code := app.Run(t.Context(), []string{"stats", "--all-projects", "--json"}); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}

	var reply response
	if err := decodeJSON(stdout.Bytes(), &reply); err != nil || !reply.OK {
		t.Fatalf("reply=%+v error=%v", reply, err)
	}

	stdout.Reset()

	if code := app.Run(t.Context(), []string{"stats", "--json"}); code != 2 || stdout.Len() != 0 {
		t.Fatalf("usage code=%d stdout=%q", code, stdout.String())
	}

	if err := decodeJSON(stderr.Bytes(), &reply); err != nil || reply.Error == nil || reply.Error.Code != "invalid_request" {
		t.Fatalf("error response=%+v error=%v", reply, err)
	}
}
