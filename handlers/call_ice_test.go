package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"testing"
	"time"
)

func TestBuildTURNServer(t *testing.T) {
	t.Setenv("TURN_URLS", "")
	if buildTURNServer(1, time.Now()) != nil {
		t.Fatal("no TURN expected without URLs")
	}

	t.Setenv("TURN_URLS", "turn:turn.example.com:3478?transport=udp, turns:turn.example.com:443?transport=tcp")
	t.Setenv("TURN_SECRET", "")
	t.Setenv("TURN_USERNAME", "")
	t.Setenv("TURN_CREDENTIAL", "")
	if buildTURNServer(1, time.Now()) != nil {
		t.Fatal("no TURN expected without credentials")
	}

	t.Setenv("TURN_USERNAME", "u")
	t.Setenv("TURN_CREDENTIAL", "p")
	s := buildTURNServer(1, time.Now())
	if s == nil || len(s.URLs) != 2 || s.Username != "u" || s.Credential != "p" {
		t.Fatalf("static creds: %+v", s)
	}

	t.Setenv("TURN_SECRET", "shh")
	now := time.Unix(1000, 0)
	s = buildTURNServer(42, now)
	wantUser := "44200:42"
	mac := hmac.New(sha1.New, []byte("shh"))
	mac.Write([]byte(wantUser))
	if s.Username != wantUser || s.Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("hmac creds: %+v", s)
	}
}
