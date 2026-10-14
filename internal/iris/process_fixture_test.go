//go:build integration || bridge

package iris

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type processFixture struct {
	binary   string
	config   config
	env      []string
	projects []string
}

type ownedProcess struct {
	cmd    *exec.Cmd
	stderr bytes.Buffer
	stdout bytes.Buffer
	exited chan struct{}
	err    error
}

func buildProcessBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "iris")
	command := exec.CommandContext(t.Context(), "go", "build", "-o", path, ".")

	command.Dir = "../.."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, output)
	}

	return path
}

func newProcessFixture(t *testing.T, binary string) processFixture {
	t.Helper()

	root := t.TempDir()
	data, configuration, runtime := filepath.Join(root, "data"), filepath.Join(root, "config"), filepath.Join(root, "runtime")

	projects := []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")}
	for _, path := range append([]string{data, configuration, runtime}, projects...) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := configFromEnvironment(environment{home: root, dataHome: data, configHome: configuration, runtimeDir: runtime})
	if err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(), "XDG_DATA_HOME="+data, "XDG_CONFIG_HOME="+configuration, "XDG_RUNTIME_DIR="+runtime)

	return processFixture{binary: binary, config: cfg, env: env, projects: projects}
}

func (fixture processFixture) command(ctx context.Context, project string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, fixture.binary, args...)
	command.Env = fixture.env
	command.Dir = project

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}

func (fixture processFixture) start(t *testing.T) *ownedProcess {
	t.Helper()

	process := &ownedProcess{exited: make(chan struct{})}
	process.cmd = exec.Command(fixture.binary, "serve")
	process.cmd.Env = fixture.env
	process.cmd.Dir = fixture.projects[0]

	process.cmd.Stderr = &process.stderr

	if err := process.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { process.err = process.cmd.Wait(); close(process.exited) }()

	t.Cleanup(func() {
		if err := process.stop(syscall.SIGTERM); err != nil {
			t.Error(err)
		}
	})
	fixture.waitReady(t, process)

	return process
}

func (process *ownedProcess) stop(signal os.Signal) error {
	select {
	case <-process.exited:
		return nil
	default:
	}

	if err := process.cmd.Process.Signal(signal); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	select {
	case <-process.exited:
		return nil
	case <-time.After(3 * time.Second):
		return process.forceStop()
	}
}

func (process *ownedProcess) forceStop() error {
	if err := process.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	select {
	case <-process.exited:
		return errors.New("owned service required forced cleanup")
	case <-time.After(3 * time.Second):
		return errors.New("owned service did not exit after cleanup")
	}
}

func (fixture processFixture) waitReady(t *testing.T, process *ownedProcess) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		stdout, _, err := fixture.command(ctx, fixture.projects[0], "stats", "--all-projects", "--json")
		if err == nil {
			var reply response
			if err := decodeJSON(stdout, &reply); err == nil && reply.OK {
				return
			}
		}

		select {
		case <-process.exited:
			t.Fatalf("service startup failed: %v %s", process.err, process.stderr.String())
		case <-ctx.Done():
			t.Fatal("service readiness timed out")
		case <-ticker.C:
		}
	}
}
