package iris

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Text, file and stdin preserve content verbatim while rejecting ambiguous or oversized input.
func TestCallInitialInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "message.txt")
	if err := os.WriteFile(path, []byte("file content\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		args        []string
		stdin, want string
		valid       bool
	}{
		{"text", []string{"--message", "text\n"}, "", "text\n", true},
		{"file", []string{"--file", path}, "", "file content\n", true},
		{"stdin", nil, "stdin\n", "stdin\n", true},
		{"ambiguous", []string{"--message", "text", "--file", path}, "", "", false},
		{"empty", []string{"--message", ""}, "", "", false},
		{"empty stdin", nil, "", "", false},
		{"large", nil, strings.Repeat("x", maxMessageBodyBytes+1), "", false},
		{"invalid UTF8", nil, string([]byte{0xff}), "", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet("call", flag.ContinueOnError)

			options := addCallFlags(flags)
			if err := flags.Parse(test.args); err != nil {
				t.Fatal(err)
			}

			app := NewApp(nil, nil)
			app.stdin = strings.NewReader(test.stdin)

			body, err := app.initialContent(flags, *options.body, *options.file)
			if (err == nil) != test.valid || body != test.want {
				t.Fatalf("body=%q error=%v", body, err)
			}
		})
	}
}

// The versioned code dictionary keeps three uniform byte-indexed words within the public grammar.
func TestInvitationCodeDictionary(t *testing.T) {
	words := strings.Fields(invitationWordsV1)

	unique := map[string]bool{}
	for _, word := range words {
		if unique[word] || !validInvitationCode(word+"-"+word+"-"+word) {
			t.Fatalf("invalid dictionary entry %q", word)
		}

		unique[word] = true
	}

	if len(words) != 256 {
		t.Fatalf("dictionary size=%d", len(words))
	}

	code, err := newInvitationCode()
	if err != nil {
		t.Fatal(err)
	}

	for _, word := range strings.Split(code, "-") {
		if !unique[word] {
			t.Fatalf("code=%q", code)
		}
	}
}

// Failed allocation rolls back implicit registration; resolving a reservation keeps the pair identity.
func TestCallProvisionalProjects(t *testing.T) {
	s, _ := testStore(t)
	alphaPath := projectDirectory(t, "alpha")
	params := callParams{projectSelection: projectSelection{CWD: alphaPath}, To: "beta", Topic: "Question", Body: "Initial text", RequestKey: "one"}

	s.makeCode = func() (string, error) { return "", problem("internal_error", "entropy unavailable") }
	if _, err := s.call(t.Context(), params); err == nil {
		t.Fatal("failed allocation accepted")
	}

	assertRegistryCount(t, s, 0)

	s.makeCode = newInvitationCode
	value := mustCall(t, s, params)

	beta, err := exactProject(t.Context(), s.db, "beta")
	if err != nil {
		t.Fatal(err)
	}

	if beta.Path != nil || beta.RegisteredAt != nil {
		t.Fatalf("recipient already registered: %+v", beta)
	}

	registered, err := s.registerProject(t.Context(), projectRegistration{Path: projectDirectory(t, "beta")})
	if err != nil || registered.ID != beta.ID {
		t.Fatalf("registered=%+v error=%v", registered, err)
	}

	if value.InitialMessage.Recipients[0] != registered.ID {
		t.Fatal("recipient identity changed on registration")
	}
}
