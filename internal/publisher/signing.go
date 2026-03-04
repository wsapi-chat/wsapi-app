package publisher

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign computes an HMAC-SHA256 signature over the body using the given secret.
func Sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
