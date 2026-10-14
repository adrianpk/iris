//go:build integration || bridge || systemd

package iris

import (
	"bytes"
	"os/exec"
	"testing"
)

func processJSON[T any](t *testing.T, fixture processFixture, path string, args ...string) T {
	t.Helper()

	stdout, stderr, err := fixture.command(t.Context(), path, args...)
	if err != nil || len(stderr) != 0 {
		t.Fatalf("command=%v error=%v stderr=%s", args, err, stderr)
	}

	return decodeProcessJSON[T](t, stdout)
}

func decodeProcessJSON[T any](t *testing.T, data []byte) T {
	t.Helper()

	jq := exec.CommandContext(t.Context(), "jq", "-e", `.protocol == "iris/v1" and .schema_version == 1 and .ok == true and .result.schema_version == 1`)

	jq.Stdin = bytes.NewReader(data)
	if output, err := jq.CombinedOutput(); err != nil {
		t.Fatalf("jq validation: %v %s", err, output)
	}

	var reply response
	if err := decodeJSON(data, &reply); err != nil {
		t.Fatal(err)
	}

	var result T
	if err := decodeJSON(reply.Result, &result); err != nil {
		t.Fatal(err)
	}

	return result
}
