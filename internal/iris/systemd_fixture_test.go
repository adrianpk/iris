//go:build systemd

package iris

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type systemdFixture struct {
	processFixture
	root  string
	name  string
	units string
}

func newSystemdFixture(t *testing.T) *systemdFixture {
	t.Helper()

	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if err := checkPrivateDirectory(runtime); err != nil {
		t.Fatal("real user-manager runtime required:", err)
	}

	root, err := os.MkdirTemp(runtime, "iris-acceptance-")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})

	fixture := &systemdFixture{root: root, name: filepath.Base(root), units: filepath.Join(root, "units")}
	fixture.configure(t)
	fixture.install(t, buildProcessBinary(t))
	t.Cleanup(func() { fixture.cleanup(t) })
	fixture.control(t, "--runtime", "link", fixture.unitPath("service"), fixture.unitPath("socket"))

	return fixture
}

func (fixture *systemdFixture) configure(t *testing.T) {
	t.Helper()

	data, configuration, runtime := filepath.Join(fixture.root, "data"), filepath.Join(fixture.root, "config"), filepath.Join(fixture.root, "runtime with % and $ signs")

	fixture.projects = []string{filepath.Join(fixture.root, "alpha"), filepath.Join(fixture.root, "beta")}
	for _, path := range append([]string{data, configuration, runtime}, fixture.projects...) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := configFromEnvironment(environment{home: fixture.root, dataHome: data, configHome: configuration, runtimeDir: runtime})
	if err != nil {
		t.Fatal(err)
	}

	fixture.config = cfg

	fixture.env = append(os.Environ(), "XDG_DATA_HOME="+data, "XDG_CONFIG_HOME="+configuration, "XDG_RUNTIME_DIR="+runtime)
	fixture.binary = filepath.Join(fixture.root, "prefix with % and $ signs", "bin/iris")
}

func (fixture *systemdFixture) install(t *testing.T, binary string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), "python3", "../../scripts/install.py", "install", "--binary", binary, "--prefix", filepath.Dir(filepath.Dir(fixture.binary)), "--unit-dir", fixture.units)

	command.Env = fixture.env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated install: %v %s", err, output)
	}

	for _, kind := range []string{"service", "socket"} {
		original := filepath.Join(fixture.units, "iris."+kind)

		content, err := os.ReadFile(original)
		if err != nil {
			t.Fatal(err)
		}

		renamed := strings.ReplaceAll(strings.ReplaceAll(string(content), "iris.service", fixture.name+".service"), "iris.socket", fixture.name+".socket")
		if err := os.WriteFile(fixture.unitPath(kind), []byte(renamed), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	command = exec.CommandContext(t.Context(), "systemd-analyze", "--user", "verify", fixture.unitPath("service"), fixture.unitPath("socket"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("verify units: %v %s", err, output)
	}
}

func (fixture *systemdFixture) unitPath(kind string) string {
	return filepath.Join(fixture.units, fixture.name+"."+kind)
}

func (fixture *systemdFixture) manager(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	output, err := command.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

func (fixture *systemdFixture) control(t *testing.T, args ...string) string {
	t.Helper()

	output, err := fixture.manager(t.Context(), args...)
	if err != nil {
		t.Fatalf("systemctl %v: %v %s", args, err, output)
	}

	return output
}

func (fixture *systemdFixture) cleanup(t *testing.T) {
	t.Helper()

	ctx := context.WithoutCancel(t.Context())
	if t.Failed() {
		command := exec.CommandContext(ctx, "journalctl", "--user", "-u", fixture.name+".service", "--no-pager", "-n", "30")
		output, err := command.CombinedOutput()
		t.Logf("owned service journal: %v %s", err, output)
	}

	fixture.resetFailed(t, ctx)

	for _, args := range [][]string{
		{"stop", fixture.name + ".socket", fixture.name + ".service"},
		{"--runtime", "disable", fixture.name + ".socket", fixture.name + ".service"},
	} {
		if output, err := fixture.manager(ctx, args...); err != nil {
			t.Errorf("owned unit cleanup %v: %v %s", args, err, output)
		}
	}
}

func (fixture *systemdFixture) pid(t *testing.T) int {
	t.Helper()

	output := fixture.control(t, "show", fixture.name+".service", "--property=MainPID", "--value")

	pid, err := strconv.Atoi(output)
	if err != nil || pid < 1 {
		t.Fatalf("invalid MainPID: %q %v", output, err)
	}

	return pid
}

func (fixture *systemdFixture) awaitRestart(t *testing.T, previous int) {
	t.Helper()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		output := fixture.control(t, "show", fixture.name+".service", "--property=MainPID", "--value")
		if output != "0" && output != fmt.Sprint(previous) {
			return
		}

		select {
		case <-deadline.C:
			t.Fatal("owned service did not restart")
		case <-ticker.C:
		}
	}
}

func (fixture *systemdFixture) resetFailed(t *testing.T, ctx context.Context) {
	t.Helper()

	for _, kind := range []string{"socket", "service"} {
		name := fixture.name + "." + kind

		state, err := fixture.manager(ctx, "is-failed", name)
		if err == nil && state == "failed" {
			if output, err := fixture.manager(ctx, "reset-failed", name); err != nil {
				t.Errorf("reset owned failed unit: %v %s", err, output)
			}
		}
	}
}
