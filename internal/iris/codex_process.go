package iris

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
)

type codexFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *codexRPCError  `json:"error"`
}

type codexRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type codexRead struct {
	frame codexFrame
	err   error
}

type codexClient struct {
	input   io.WriteCloser
	frames  <-chan codexRead
	done    <-chan struct{}
	cmd     *exec.Cmd
	nextID  int
	cancel  context.CancelFunc
	approve codexApproval
}

func startCodex(ctx context.Context, path string) (*codexClient, error) {
	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, "codex", "app-server", "--listen", "stdio://")
	cmd.Dir, cmd.Stderr = path, io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second

	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()

		return nil, err
	}

	output, err := cmd.StdoutPipe()
	if err != nil {
		cancel()

		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()

		return nil, problem("worker_failed", "cannot start the installed Codex app-server")
	}

	frames, done := readCodexFrames(processCtx, output, cmd)

	return &codexClient{input: input, frames: frames, done: done, cmd: cmd, cancel: cancel}, nil
}

// One queued frame applies backpressure instead of retaining an unbounded event history.
func readCodexFrames(ctx context.Context, output io.Reader, cmd *exec.Cmd) (<-chan codexRead, <-chan struct{}) {
	frames, done := make(chan codexRead, 1), make(chan struct{})
	go func() {
		defer close(done)
		defer close(frames)

		scanCodexFrames(ctx, output, frames)

		// The parent receives EOF as an uncertain outcome; raw provider diagnostics stay private.
		if err := cmd.Wait(); err != nil {
			return
		}
	}()

	return frames, done
}

func scanCodexFrames(ctx context.Context, output io.Reader, frames chan<- codexRead) {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), maxResponseBytes)

	for scanner.Scan() {
		var value codexFrame

		err := json.Unmarshal(scanner.Bytes(), &value)
		select {
		case frames <- codexRead{frame: value, err: err}:
		case <-ctx.Done():
			return
		}
	}
}

func (client *codexClient) close() error {
	if client.cancel != nil {
		client.cancel()
	}

	inputErr := client.input.Close()
	if client.cmd == nil {
		return inputErr
	}

	signalErr := signalCodex(client.cmd, syscall.SIGTERM)
	select {
	case <-client.done:
	case <-time.After(2 * time.Second):
		signalErr = errors.Join(signalErr, signalCodex(client.cmd, syscall.SIGKILL))
		<-client.done
	}

	return errors.Join(inputErr, signalErr)
}

func signalCodex(cmd *exec.Cmd, signal syscall.Signal) error {
	err := syscall.Kill(-cmd.Process.Pid, signal)
	if err == syscall.ESRCH {
		return nil
	}

	return err
}
