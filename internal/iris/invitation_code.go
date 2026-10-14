package iris

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"strings"
)

//go:embed invitation_words_v1.txt
var invitationWordsV1 string

func newInvitationCode() (string, error) {
	words := strings.Fields(invitationWordsV1)

	var indices [3]byte
	if _, err := rand.Read(indices[:]); err != nil {
		return "", err
	}

	return words[indices[0]] + "-" + words[indices[1]] + "-" + words[indices[2]], nil
}

func textDigest(value string) string {
	digest := sha256.Sum256([]byte(value))

	return hex.EncodeToString(digest[:])
}

func validInvitationCode(code string) bool {
	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if len(part) < 1 || len(part) > 16 {
			return false
		}

		for _, letter := range part {
			if letter < 'a' || letter > 'z' {
				return false
			}
		}
	}

	return true
}
