package iris

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func (app *App) bridgeTerminal(options bridgeOptions) (*os.File, error) {
	if options.Approvals != "interactive" {
		return nil, nil
	}

	file, ok := app.stdin.(*os.File)
	if !ok {
		return nil, problem("invalid_request", "interactive approvals require a connected terminal")
	}

	if _, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS); err != nil {
		return nil, problem("invalid_request", "interactive approvals require a connected terminal")
	}

	return file, nil
}

func (app *App) terminalApproval(file *os.File) codexApproval {
	return func(ctx context.Context, frame codexFrame) (bool, error) {
		if len(frame.Params) > 32768 {
			return false, problem("limit_exceeded", "approval display exceeds the local request limit")
		}

		if _, err := fmt.Fprintf(app.stderr, "Codex approval %s: %s\nType approve to accept this request once; anything else declines: ", frame.Method, frame.Params); err != nil {
			return false, err
		}

		answer, err := readApproval(ctx, file)

		return strings.TrimSpace(answer) == "approve", err
	}
}

func readApproval(ctx context.Context, file *os.File) (string, error) {
	var answer []byte
	for len(answer) < 128 {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		ready, err := approvalReady(file)
		if err != nil {
			return "", err
		}

		if !ready {
			continue
		}

		var value [1]byte

		n, err := unix.Read(int(file.Fd()), value[:])
		if err != nil || n == 0 {
			return "", problem("approval_required", "approval terminal disconnected")
		}

		if value[0] == '\n' {
			return string(answer), nil
		}

		answer = append(answer, value[0])
	}

	return "", problem("approval_required", "approval response exceeds the local limit")
}

func approvalReady(file *os.File) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(file.Fd()), Events: unix.POLLIN}}

	_, err := unix.Poll(fds, 100)
	if err == unix.EINTR {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		return false, problem("approval_required", "approval terminal disconnected")
	}

	return fds[0].Revents&unix.POLLIN != 0, nil
}
