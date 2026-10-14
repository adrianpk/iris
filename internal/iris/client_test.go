package iris

import (
	"errors"
	"net"
	"testing"
)

// A malformed response is never exposed as successful CLI JSON.
func TestClientInvalidResponses(t *testing.T) {
	cases := []struct{ name, data string }{
		{"version", `{"protocol":"other/v1","schema_version":1,"ok":true,"result":{}}`},
		{"schema", `{"protocol":"iris/v1","schema_version":2,"ok":true,"result":{}}`},
		{"both", `{"protocol":"iris/v1","schema_version":1,"ok":true,"result":{},"error":{"code":"x","message":"error","retryable":false}}`},
		{"missing result", `{"protocol":"iris/v1","schema_version":1,"ok":true}`},
		{"missing error", `{"protocol":"iris/v1","schema_version":1,"ok":false}`},
		{"unknown", `{"protocol":"iris/v1","schema_version":1,"ok":true,"result":{},"extra":1}`},
		{"malformed", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, peer := net.Pipe()
			done := make(chan error, 1)

			go func() { _, err := peer.Write([]byte(tc.data + "\n")); done <- errors.Join(err, peer.Close()) }()

			_, err := readResponse(client)
			if closeErr := client.Close(); closeErr != nil {
				t.Error(closeErr)
			}

			if writerErr := <-done; writerErr != nil {
				t.Error(writerErr)
			}

			if err == nil {
				t.Fatal("malformed response accepted")
			}
		})
	}
}
