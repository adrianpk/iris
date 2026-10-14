package iris

import (
	"os"
	"path/filepath"
	"testing"
)

// Resolve XDG roots independently of cwd and reject relative or missing paths.
func TestConfigurationPaths(t *testing.T) {
	cases := []struct {
		name  string
		env   environment
		valid bool
		data  string
	}{
		{"defaults", environment{home: "/users/test", runtimeDir: "/run/test"}, true, "/users/test/.local/share/iris"},
		{"overrides", environment{home: "/users/test", dataHome: "/data", configHome: "/config", runtimeDir: "/run/test"}, true, "/data/iris"},
		{"runtime missing", environment{home: "/users/test"}, false, ""},
		{"relative data", environment{home: "/users/test", dataHome: "relative", runtimeDir: "/run/test"}, false, ""},
		{"relative config", environment{home: "/users/test", configHome: "relative", runtimeDir: "/run/test"}, false, ""},
		{"relative runtime", environment{home: "/users/test", runtimeDir: "relative"}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := configFromEnvironment(tc.env)
			if (err == nil) != tc.valid {
				t.Fatalf("configuration error=%v", err)
			}

			if tc.valid && cfg.dataRoot != tc.data {
				t.Fatalf("data root=%q", cfg.dataRoot)
			}
		})
	}
}

// Configuration accepts documented overrides and rejects malformed or unknown input.
func TestConfigurationFile(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		valid bool
	}{
		{"partial", `{"max_connections":4}`, true},
		{"version", `{"schema_version":2}`, false},
		{"zero connections", `{"max_connections":0}`, false},
		{"too many", `{"max_connections":129}`, false},
		{"timeout", `{"request_timeout_seconds":0}`, false},
		{"unknown", `{"other":1}`, false},
		{"trailing", `{} {}`, false},
		{"malformed", `{`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config{configPath: filepath.Join(t.TempDir(), "config.json"), settings: settings{1, 128, 10}}
			if err := os.WriteFile(cfg.configPath, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := cfg.loadSettings(); (err == nil) != tc.valid {
				t.Fatalf("load error=%v", err)
			}
		})
	}
}
