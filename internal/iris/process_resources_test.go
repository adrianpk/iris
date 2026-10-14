//go:build bridge || systemd

package iris

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

type idleResources struct {
	ticks uint64
	rss   int64
}

func readIdleProcess(t *testing.T, pid int) idleResources {
	t.Helper()

	root := "/proc/" + strconv.Itoa(pid)

	data, err := os.ReadFile(root + "/stat")
	if err != nil {
		t.Fatal(err)
	}

	fields := strings.Fields(string(data)[strings.LastIndex(string(data), ")")+1:])

	user, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	system, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	status, err := os.ReadFile(root + "/status")
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			rss, err := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}

			return idleResources{ticks: user + system, rss: rss}
		}
	}

	t.Fatal("owned process RSS is unavailable")

	return idleResources{}
}
