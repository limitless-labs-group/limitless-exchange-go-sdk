package limitless

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// HMACCredentials holds a scoped API token's identity and secret.
type HMACCredentials struct {
	TokenID string
	Secret  string
}

func buildHMACMessage(timestamp, method, path, body string) string {
	return timestamp + "\n" + strings.ToUpper(method) + "\n" + path + "\n" + body
}

func computeHMACSignature(secret, timestamp, method, path, body string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("failed to decode HMAC secret: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte(buildHMACMessage(timestamp, method, path, body))); err != nil {
		return "", fmt.Errorf("failed to write HMAC message: %w", err)
	}

	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
