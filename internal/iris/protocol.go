package iris

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	protocolVersion  = "iris/v1"
	maxRequestBytes  = 1 << 20
	maxResponseBytes = 8 << 20
)

var errFrameLimit = errors.New("encoded frame exceeds limit")

type request struct {
	Protocol  string          `json:"protocol"`
	Operation string          `json:"operation"`
	Params    json.RawMessage `json:"params"`
}

type response struct {
	Protocol      string          `json:"protocol"`
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Result        json.RawMessage `json:"result,omitempty"`
	Error         *serviceError   `json:"error,omitempty"`
}

type serviceError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	cause     error
}

func (err *serviceError) Error() string {
	return err.Code + ": " + err.Message
}

func (err *serviceError) Unwrap() error {
	return err.cause
}

func failure(code, message string, retryable bool) response {
	return response{Protocol: protocolVersion, SchemaVersion: 1, Error: &serviceError{Code: code, Message: message, Retryable: retryable}}
}

func success(result any) response {
	encoded, err := json.Marshal(result)
	if err != nil {
		return failure("internal_error", "cannot encode service result", false)
	}

	return response{Protocol: protocolVersion, SchemaVersion: 1, OK: true, Result: encoded}
}

func readFrame(reader io.Reader, maximum int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(maximum)+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maximum {
		return nil, errFrameLimit
	}

	if len(data) == 0 || data[len(data)-1] != '\n' || bytes.Count(data, []byte{'\n'}) != 1 {
		return nil, fmt.Errorf("expected one newline-terminated JSON frame followed by EOF")
	}

	return data, nil
}

func writeFrame(writer io.Writer, value any, maximum int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if len(data)+1 > maximum {
		return errFrameLimit
	}

	data = append(data, '\n')

	n, err := writer.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}

	return err
}
