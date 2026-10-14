package iris

import (
	"bytes"
	"fmt"
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

	if code := app.Run(t.Context(), []string{"stats", "--json"}); code != 1 || stdout.Len() != 0 {
		t.Fatalf("usage code=%d stdout=%q", code, stdout.String())
	}

	if err := decodeJSON(stderr.Bytes(), &reply); err != nil || reply.Error == nil || reply.Error.Code != "project_not_found" {
		t.Fatalf("error response=%+v error=%v", reply, err)
	}
}

// Daemon stderr exposes the startup cause for operators without changing RPC errors.
func TestServeDiagnostic(t *testing.T) {
	var stdout, stderr bytes.Buffer

	app := NewApp(&stdout, &stderr)

	app.config = func() (config, error) { return config{}, fmt.Errorf("configuration field outside allowed range") }
	if code := app.Run(t.Context(), []string{"serve"}); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "configuration field outside allowed range") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	cfg, _ := testService(t, nil)
	app.config = func() (config, error) { return cfg, nil }

	stderr.Reset()

	if code := app.Run(t.Context(), []string{"serve"}); code != 1 || !strings.Contains(stderr.String(), "service already owns this runtime") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}
