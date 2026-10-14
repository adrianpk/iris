package iris

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// A second owner cannot replace the first daemon's lock or socket.
func TestRuntimeOwnership(t *testing.T) {
	cfg, _ := testService(t, nil)
	if lock, err := acquireRuntime(cfg); err == nil {
		if err := lock.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal("second owner accepted")
	}

	if listener, err := listenUnix(cfg.socket); err == nil {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal("active socket replaced")
	}

	if _, err := callService(t.Context(), cfg, statsRequest()); err != nil {
		t.Fatal("first service damaged:", err)
	}
}

// Only a private stale socket is reclaimed; regular files remain untouched.
func TestRuntimeStaleSocket(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "stale.sock")

	createStaleSocket(t, path)

	listener, err := listenUnix(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}

	if listener, err := listenUnix(path); err == nil {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal("regular file replaced")
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("data=%q error=%v", data, err)
	}
}

// Clients reject a socket whose permissions no longer satisfy the local boundary.
func TestClientSocketPermissions(t *testing.T) {
	cfg, _ := testService(t, nil)
	if err := os.Chmod(cfg.socket, 0o666); err != nil {
		t.Fatal(err)
	}

	if _, err := callService(t.Context(), cfg, statsRequest()); err == nil {
		t.Fatal("unsafe socket accepted")
	}
}

func createStaleSocket(t *testing.T, path string) {
	t.Helper()

	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}

	stale.SetUnlinkOnClose(false)

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
}
