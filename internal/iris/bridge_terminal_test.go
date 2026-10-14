package iris

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Interactive authority cannot be acquired through a peer pipe or an unattended CLI invocation.
func TestBridgeRequiresTerminal(t *testing.T) {
	app := NewApp(&bytes.Buffer{}, &bytes.Buffer{})
	app.stdin = strings.NewReader("approve\n")

	app.config = func() (config, error) { t.Fatal("configuration accessed before authority check"); return config{}, nil }
	if err := app.runBridgeStart(t.Context(), []string{"--approvals", "interactive", strings.Repeat("a", 32)}); !hasCode(err, "invalid_request") {
		t.Fatalf("unattended interactive mode=%v", err)
	}
}

// A connected local decision is exact, bounded, and goes to diagnostics without polluting JSON stdout.
func TestBridgeLocalDecision(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := read.Close(); err != nil {
			t.Error(err)
		}

		if err := write.Close(); err != nil {
			t.Error(err)
		}
	})

	if _, err := write.WriteString("approve\n"); err != nil {
		t.Fatal(err)
	}

	var output, diagnostics bytes.Buffer

	app := NewApp(&output, &diagnostics)

	accepted, err := app.terminalApproval(read)(t.Context(), codexFrame{Method: "item/commandExecution/requestApproval", Params: []byte(`{"command":"bounded local work"}`)})
	if err != nil || !accepted || output.Len() != 0 || !strings.Contains(diagnostics.String(), "once") {
		t.Fatalf("accepted=%v, diagnostics=%s, error=%v", accepted, diagnostics.String(), err)
	}
}
