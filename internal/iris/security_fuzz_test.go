package iris

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

// Framing and strict request decoding must preserve one bounded UTF-8 JSON object without panics.
func FuzzIrisFrame(f *testing.F) {
	for _, seed := range []string{"{\"protocol\":\"iris/v1\",\"operation\":\"stats\",\"params\":{\"all_projects\":true}}\n", "{} {}\n", "{\"operation\":\"\xff\"}\n", "[]\n"} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}

		checkIrisFuzzFrame(t, data)
	})
}

// Arbitrary stored provider histories cannot escape a recorded thread/turn or return malformed successful output.
func FuzzCodexRecovery(f *testing.F) {
	f.Add([]byte(storedCodexFixture("completed", "own-turn", "/fixture/project", "completed")))
	f.Add([]byte(`{"thread":{}}`))
	f.Add([]byte("\xff"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			return
		}

		thread, turn := "own-thread", "own-turn"

		value, err := recoverCodexResult(data, bridgeBinding{ThreadID: &thread, TurnID: &turn, ProjectPath: "/fixture/project"})
		if err != nil {
			return
		}

		if !utf8.Valid(data) || value.TurnID != turn {
			t.Fatal("foreign or invalid history accepted")
		}

		if value.Answer.Action == "ack" && value.Answer.Body == "" {
			return
		}

		if value.Answer.Action != "reply" || !validText(value.Answer.Body, maxMessageBodyBytes) {
			t.Fatal("malformed successful answer")
		}
	})
}

func checkIrisFuzzFrame(t *testing.T, data []byte) {
	t.Helper()

	frame, err := readFrame(bytes.NewReader(data), maxRequestBytes)
	if err != nil {
		return
	}

	var req request
	if decodeJSON(frame, &req) != nil {
		return
	}

	if !utf8.Valid(frame) || !json.Valid(frame) || bytes.Count(frame, []byte{'\n'}) != 1 {
		t.Fatal("invalid frame accepted")
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var roundtrip request
	if decodeJSON(encoded, &roundtrip) != nil || req.Protocol != roundtrip.Protocol || req.Operation != roundtrip.Operation {
		t.Fatal("request scope changed on round trip")
	}
}
