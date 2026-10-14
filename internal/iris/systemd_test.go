//go:build systemd

package iris

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Real user units activate on request, preserve state/socket ownership and stop cleanly.
func TestSystemdOperation(t *testing.T) {
	fixture := newSystemdFixture(t)
	fixture.control(t, "start", fixture.name+".socket")

	if state := fixture.control(t, "show", fixture.name+".service", "--property=ActiveState", "--value"); state != "inactive" {
		t.Fatalf("service started without request: %s", state)
	}

	if err := checkSocket(fixture.config.socket); err != nil {
		t.Fatal(err)
	}

	socket, err := os.Lstat(fixture.config.socket)
	if err != nil {
		t.Fatal(err)
	}

	initial := processJSON[statistics](t, fixture.processFixture, fixture.projects[0], "stats", "--all-projects", "--json")
	if !initial.Ready || initial.Projects != 0 {
		t.Fatalf("initial=%+v", initial)
	}

	pid := fixture.pid(t)
	fixture.assertRestrictions(t, pid)

	args := []string{"call", "--to", "beta", "--topic", "systemd persistence", "--message", "Retain this request", "--idempotency-key", "systemd", "--json"}
	original := processJSON[conversation](t, fixture.processFixture, fixture.projects[0], args...)
	processJSON[conversation](t, fixture.processFixture, fixture.projects[1], "join", "--json", original.InvitationCode)
	fixture.assertPrivateFiles(t)
	fixture.assertIdle(t, pid)
	fixture.verifyDisconnect(t, original.ID)
	fixture.install(t, fixture.binary)

	if fixture.pid(t) != pid {
		t.Fatal("file installation restarted service implicitly")
	}

	fixture.control(t, "daemon-reload")
	fixture.control(t, "restart", fixture.name+".service")
	fixture.awaitRestart(t, pid)
	fixture.assertRetained(t, socket, args, original.ID)

	pid = fixture.pid(t)
	fixture.control(t, "kill", "--signal=KILL", "--kill-whom=main", fixture.name+".service")
	fixture.awaitRestart(t, pid)
	fixture.assertRetained(t, socket, args, original.ID)
	fixture.verifyShutdown(t, original.ID)
	fixture.assertRetained(t, socket, args, original.ID)
	fixture.control(t, "stop", fixture.name+".socket", fixture.name+".service")

	if _, err := os.Lstat(fixture.config.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stopped manager socket remains: %v", err)
	}

	fixture.assertClosedStore(t)
}

func (fixture *systemdFixture) assertRestrictions(t *testing.T, pid int) {
	t.Helper()

	status, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{"NoNewPrivs:\t1", "CapEff:\t0000000000000000"} {
		if !strings.Contains(string(status), field) {
			t.Fatalf("missing daemon restriction %s", field)
		}
	}

	children, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/task/" + strconv.Itoa(pid) + "/children")
	if err != nil || len(strings.TrimSpace(string(children))) != 0 {
		t.Fatalf("unexpected daemon child: %s %v", children, err)
	}
}

func (fixture *systemdFixture) assertPrivateFiles(t *testing.T) {
	t.Helper()

	for _, path := range []string{fixture.config.dataRoot, fixture.config.runtimeRoot} {
		if err := checkPrivateDirectory(path); err != nil {
			t.Fatal(path, err)
		}
	}

	for _, path := range []string{fixture.config.database, fixture.config.database + "-wal", fixture.config.database + "-shm", fixture.config.socket + ".lock"} {
		if err := privateRegularFile(path); err != nil {
			t.Fatal(path, err)
		}
	}
}

func (fixture *systemdFixture) assertIdle(t *testing.T, pid int) {
	t.Helper()

	before := readIdleProcess(t, pid)
	started := time.Now()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}

	after := readIdleProcess(t, pid)
	fixture.assertRestrictions(t, pid)
	t.Logf("idle daemon: interval_ms=%d cpu_ticks=%d rss_kib=%d; no bridge/provider children", time.Since(started).Milliseconds(), after.ticks-before.ticks, after.rss)
}

func (fixture *systemdFixture) assertRetained(t *testing.T, socket os.FileInfo, args []string, id string) {
	t.Helper()

	value := processJSON[conversation](t, fixture.processFixture, fixture.projects[0], args...)
	if value.ID != id || value.InitialMessage.Body != "Retain this request" {
		t.Fatalf("state lost after restart: %+v", value)
	}

	current, err := os.Lstat(fixture.config.socket)
	if err != nil || !os.SameFile(socket, current) {
		t.Fatalf("manager socket replaced: %v", err)
	}

	if err := checkSocket(fixture.config.socket); err != nil {
		t.Fatal(err)
	}
}

func (fixture *systemdFixture) verifyDisconnect(t *testing.T, id string) {
	t.Helper()

	page := processJSON[receivePage](t, fixture.processFixture, fixture.projects[1], "receive", "--consumer", "systemd", "--wait", "0", "--json", id)
	for _, message := range page.Messages {
		processJSON[consumerPosition](t, fixture.processFixture, fixture.projects[1], "ack", "--consumer", "systemd", "--message-id", message.ID, "--json", id)
	}

	connection := fixture.pendingReceive(t, id)
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	page = processJSON[receivePage](t, fixture.processFixture, fixture.projects[1], "receive", "--consumer", "systemd", "--wait", "1", "--json", id)
	if len(page.Messages) != 0 {
		t.Fatal("disconnect changed processing state")
	}
}

func (fixture *systemdFixture) pendingReceive(t *testing.T, id string) *net.UnixConn {
	t.Helper()

	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: fixture.config.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})

	params := receiveParams{conversationParams: conversationParams{projectSelection: projectSelection{CWD: fixture.projects[1]}, ConversationID: id}, ConsumerID: "systemd", WaitSeconds: 60}

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeFrame(connection, request{Protocol: protocolVersion, Operation: "receive", Params: encoded}, maxRequestBytes); err != nil {
		t.Fatal(err)
	}

	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	assertPendingSystemdRead(t, connection)

	return connection
}

func assertPendingSystemdRead(t *testing.T, connection *net.UnixConn) {
	t.Helper()

	if err := connection.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	var probe [1]byte

	_, err := connection.Read(probe[:])

	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("long wait completed early: %v", err)
	}

	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
}

func (fixture *systemdFixture) verifyShutdown(t *testing.T, id string) {
	t.Helper()

	pending := fixture.pendingReceive(t, id)
	fixture.control(t, "stop", fixture.name+".service")

	if result := fixture.control(t, "show", fixture.name+".service", "--property=Result", "--value"); result != "success" {
		t.Fatalf("unclean service shutdown: %s", result)
	}

	if data, err := readFrame(pending, maxResponseBytes); err == nil {
		var reply response
		if err := decodeJSON(data, &reply); err != nil || reply.OK {
			t.Fatalf("cancelled receive response=%s error=%v", data, err)
		}
	}

	if err := checkSocket(fixture.config.socket); err != nil {
		t.Fatal("service removed manager socket:", err)
	}
}

func (fixture *systemdFixture) assertClosedStore(t *testing.T) {
	t.Helper()

	reopened, err := openStore(t.Context(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	var integrity string

	queryErr := reopened.db.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&integrity)
	if err := errors.Join(queryErr, reopened.close()); err != nil || integrity != "ok" {
		t.Fatalf("closed database integrity=%s error=%v", integrity, err)
	}

	if err := privateRegularFile(fixture.config.database); err != nil {
		t.Fatal(err)
	}
}
