package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type iceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

const turnCredentialTTL = 12 * time.Hour

// GetCallICEServers GET /api/calls/ice-servers
// Returns STUN plus the configured TURN relay. Without TURN, calls between users behind
// strict NAT or corporate firewalls connect slowly, one way, or not at all.
func GetCallICEServers(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	servers := []iceServer{{URLs: []string{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"}}}

	turn := buildTURNServer(userID, time.Now())
	if turn != nil {
		servers = append(servers, *turn)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"iceServers":     servers,
		"turnConfigured": turn != nil,
		"ttlSeconds":     int(turnCredentialTTL.Seconds()),
	})
}

func buildTURNServer(userID int, now time.Time) *iceServer {
	urls := []string{}
	for _, u := range strings.Split(os.Getenv("TURN_URLS"), ",") {
		if u = strings.TrimSpace(u); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return nil
	}

	if secret := os.Getenv("TURN_SECRET"); secret != "" {
		// coturn REST API scheme: username = "<expiry>:<user>", credential = base64(HMAC-SHA1(secret, username)).
		username := fmt.Sprintf("%d:%d", now.Add(turnCredentialTTL).Unix(), userID)
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write([]byte(username))
		return &iceServer{URLs: urls, Username: username, Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil))}
	}

	username, credential := os.Getenv("TURN_USERNAME"), os.Getenv("TURN_CREDENTIAL")
	if username == "" || credential == "" {
		return nil
	}
	return &iceServer{URLs: urls, Username: username, Credential: credential}
}
