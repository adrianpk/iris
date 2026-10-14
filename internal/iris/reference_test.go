package iris

import (
	"os"
	"regexp"
	"testing"
)

// The published complete configuration must remain valid and equal to executable defaults.
func TestReferenceConfiguration(t *testing.T) {
	document, err := os.ReadFile("../../docs/reference/runtime.md")
	if err != nil {
		t.Fatal(err)
	}

	block := regexp.MustCompile("(?s)```json\n(.*?)```").FindSubmatch(document)
	if len(block) != 2 {
		t.Fatal("configuration example missing")
	}

	var example settings
	if err := decodeJSON(block[1], &example); err != nil {
		t.Fatal(err)
	}

	if err := example.validate(); err != nil {
		t.Fatal(err)
	}

	if example != defaultSettings() {
		t.Fatalf("documented defaults=%+v actual=%+v", example, defaultSettings())
	}
}
