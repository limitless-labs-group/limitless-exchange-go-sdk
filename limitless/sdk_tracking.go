package limitless

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

const sdkID = "lmts-sdk-go"

// sdkVersion is injected at release build time via -ldflags.
var sdkVersion = "1.1.0"

func resolveSDKVersion() string {
	if sdkVersion != "" {
		return sdkVersion
	}
	return "0.0.0"
}

func sdkToken() string {
	return fmt.Sprintf("%s/%s", sdkID, resolveSDKVersion())
}

func sdkTrackingHeaders() map[string]string {
	return map[string]string{
		"user-agent":    fmt.Sprintf("%s (go/%s)", sdkToken(), runtime.Version()),
		"x-sdk-version": sdkToken(),
	}
}

func buildWebSocketHeaders(apiKey string, hmacCreds *HMACCredentials) (http.Header, error) {
	headers := http.Header{}
	for key, value := range sdkTrackingHeaders() {
		headers.Set(key, value)
	}

	if hmacCreds != nil {
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		signature, err := computeHMACSignature(hmacCreds.Secret, timestamp, http.MethodGet, "/socket.io/?EIO=4&transport=websocket", "")
		if err != nil {
			return nil, fmt.Errorf("failed to compute websocket HMAC signature: %w", err)
		}
		headers.Set("lmts-api-key", hmacCreds.TokenID)
		headers.Set("lmts-timestamp", timestamp)
		headers.Set("lmts-signature", signature)
	} else if apiKey != "" {
		headers.Set("X-API-Key", apiKey)
	}

	return headers, nil
}
