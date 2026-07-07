package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifySignature verifies a GitHub webhook payload against its HMAC-SHA256 signature.
// The signature is expected in the format "sha256=<hex>".
func VerifySignature(payload []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sigHex := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sigHex), []byte(expected))
}
