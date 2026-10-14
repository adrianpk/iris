package iris

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

func parseCodexFrame(data []byte) (codexFrame, error) {
	var value codexFrame

	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' || len(data) > maxResponseBytes || !utf8.Valid(data) {
		return value, fmt.Errorf("invalid app-server JSON frame")
	}

	err := json.Unmarshal(data, &value)

	return value, err
}
