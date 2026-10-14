package iris

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Help must be usable before runtime configuration or a daemon exists.
func TestHelpWithoutService(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			app := NewApp(&stdout, &stderr)
			app.config = func() (config, error) { t.Fatal("help resolved configuration"); return config{}, nil }

			if code := app.Run(t.Context(), args); code != 0 || !strings.Contains(stdout.String(), "Usage: iris") || stderr.Len() != 0 {
				t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

// Invalid commands and cancellation have distinct exits and clean stdout.
func TestCommandFailure(t *testing.T) {
	cases := []struct {
		name      string
		cancelled bool
		args      []string
		code      int
	}{
		{"unknown", false, []string{"unknown"}, 2},
		{"cancelled", true, []string{"unknown"}, 130},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if tc.cancelled {
				cancel()
			}

			var stdout, stderr bytes.Buffer
			if code := NewApp(&stdout, &stderr).Run(ctx, tc.args); code != tc.code || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
