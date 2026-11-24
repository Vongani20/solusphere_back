package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type attemptWindow struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (w *attemptWindow) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	cutoff := now.Add(-window)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hits == nil {
		w.hits = map[string][]time.Time{}
	}
	recent := w.hits[key][:0]
	for _, at := range w.hits[key] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	if len(recent) >= limit {
		w.hits[key] = recent
		return false
	}
	w.hits[key] = append(recent, now)
	if len(w.hits) > 4000 {
		for item, times := range w.hits {
			kept := times[:0]
			for _, at := range times {
				if at.After(cutoff) {
					kept = append(kept, at)
				}
			}
			if len(kept) == 0 {
				delete(w.hits, item)
			} else {
				w.hits[item] = kept
			}
		}
	}
	return true
}

var authAttempts attemptWindow

// LimitJSONField slows repeated attempts against the same account.
// The request body is put back so the handler can read it.
func LimitJSONField(field string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		key := "missing"
		var payload map[string]json.RawMessage
		if json.Unmarshal(body, &payload) == nil {
			var value string
			if raw, ok := payload[field]; ok && json.Unmarshal(raw, &value) == nil {
				value = strings.ToLower(strings.TrimSpace(value))
				if value != "" {
					key = value
				}
			}
		}
		if !authAttempts.allow(c.FullPath()+"|"+field+"|"+window.String()+"|"+key, limit, window) {
			TooManyRequests(c)
			return
		}
		c.Next()
	}
}
