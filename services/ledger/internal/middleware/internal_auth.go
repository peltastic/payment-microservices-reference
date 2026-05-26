package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	timestampHeader  = "x-internal-timestamp"
	signatureHeader  = "x-internal-signature"
	bodyHashHeader   = "x-internal-body-sha256"
	requestIDHeader  = "x-request-id"
	merchantIDHeader = "x-merchant-id"
	maxClockSkew     = 5 * time.Minute
	maxBodyBytes     = 1 << 20
)

func RequireInternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := strings.TrimSpace(os.Getenv("INTERNAL_AUTH_SECRET"))
		timestamp := strings.TrimSpace(c.GetHeader(timestampHeader))
		signature := strings.TrimSpace(c.GetHeader(signatureHeader))
		providedBodyHash := strings.TrimSpace(c.GetHeader(bodyHashHeader))
		body, ok := requestBody(c)
		if !ok {
			abortInternalAuth(c)
			return
		}
		bodyHash := sha256.Sum256(body)
		actualBodyHash := hex.EncodeToString(bodyHash[:])

		if secret == "" || !validTimestamp(timestamp) || signature == "" || providedBodyHash == "" || !hmac.Equal([]byte(providedBodyHash), []byte(actualBodyHash)) {
			abortInternalAuth(c)
			return
		}

		canonical := strings.Join([]string{
			strings.ToUpper(c.Request.Method),
			c.Request.URL.RequestURI(),
			timestamp,
			strings.TrimSpace(c.GetHeader(requestIDHeader)),
			strings.TrimSpace(c.GetHeader(merchantIDHeader)),
			actualBodyHash,
		}, "\n")

		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(canonical))
		expected := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expected)) {
			abortInternalAuth(c)
			return
		}

		c.Next()
	}
}

func requestBody(c *gin.Context) ([]byte, bool) {
	if c.Request.Body == nil {
		return []byte{}, true
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return nil, false
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, true
}

func validTimestamp(value string) bool {
	unixSeconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false
	}

	timestamp := time.Unix(unixSeconds, 0)
	now := time.Now()

	return timestamp.After(now.Add(-maxClockSkew)) && timestamp.Before(now.Add(maxClockSkew))
}

func abortInternalAuth(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":    "invalid_internal_signature",
			"message": "Internal authentication failed",
		},
	})
}
