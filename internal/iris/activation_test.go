package iris

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// Activation metadata fails closed instead of replacing a manager-owned socket.
func TestActivationMetadata(t *testing.T) {
	tests := []struct {
		name, pid, count, names string
		active, rejected        bool
	}{
		{name: "manual"},
		{name: "named", pid: "42", count: "1", names: "iris", active: true},
		{name: "unnamed", pid: "42", count: "1", active: true},
		{name: "other process", pid: "41", count: "1", rejected: true},
		{name: "missing owner", count: "1", rejected: true},
		{name: "invalid owner", pid: "bad", count: "1", rejected: true},
		{name: "missing count", pid: "42", rejected: true},
		{name: "zero", pid: "42", count: "0", rejected: true},
		{name: "multiple", pid: "42", count: "2", rejected: true},
		{name: "other descriptor", pid: "42", count: "1", names: "other", rejected: true},
		{name: "multiple names", pid: "42", count: "1", names: "iris:other", rejected: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			active, err := parseActivation(42, test.pid, test.count, test.names)
			if active != test.active || (err != nil) != test.rejected {
				t.Fatalf("active=%v error=%v", active, err)
			}
		})
	}
}

// Adopting a descriptor preserves the manager's socket across daemon shutdown.
func TestActivationOwnership(t *testing.T) {
	path := activationPath(t)

	original, err := listenUnix(path)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	}()

	file, err := original.File()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()

	adopted, err := inheritedListener(path, file)
	if err != nil {
		t.Fatal(err)
	}

	if err := adopted.Close(); err != nil {
		t.Fatal(err)
	}

	if err := checkSocket(path); err != nil {
		t.Fatal("manager socket removed:", err)
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal("manager listener damaged:", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

// Wrong paths, unsafe modes, regular files and network sockets cannot be adopted.
func TestActivationDescriptors(t *testing.T) {
	tests := []struct {
		name      string
		file      func(*testing.T, string) *os.File
		mode      os.FileMode
		wrongPath bool
	}{
		{name: "wrong path", file: activationUnixFile, mode: 0o600, wrongPath: true},
		{name: "public socket", file: activationUnixFile, mode: 0o666},
		{name: "regular file", file: activationRegularFile},
		{name: "network listener", file: activationTCPFile},
		{name: "packet listener", file: activationPacketFile},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := activationPath(t)

			file := test.file(t, path)
			activationMode(t, path, test.mode)

			expected := path
			if test.wrongPath {
				expected += ".wrong"
			}

			listener, err := inheritedListener(expected, file)
			if err == nil {
				if err := listener.Close(); err != nil {
					t.Error(err)
				}

				t.Fatal("invalid descriptor accepted")
			}
		})
	}
}

func activationUnixFile(t *testing.T, path string) *os.File {
	t.Helper()

	listener, err := listenUnix(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})

	file, err := listener.File()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})

	return file
}

func activationRegularFile(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})

	return file
}

func activationTCPFile(t *testing.T, _ string) *os.File {
	t.Helper()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})

	file, err := listener.File()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})

	return file
}

func activationPath(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "private")
	if err := privateDirectory(root); err != nil {
		t.Fatal(err)
	}

	return filepath.Join(root, "iris.sock")
}

func activationMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	if mode == 0 {
		return
	}

	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func activationPacketFile(t *testing.T, path string) *os.File {
	t.Helper()

	listener, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})

	file, err := listener.File()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})

	return file
}
