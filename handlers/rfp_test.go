package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRFPSourcesFromJSONUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	text := "REQUEST FOR PROPOSAL\nScope:\n- Provide interpreting services at hearings\n"
	body, _ := json.Marshal(map[string]interface{}{
		"documents": []map[string]string{{
			"filename":     "document.txt",
			"display_name": "Tender notice",
			"document":     "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(text)),
		}},
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/rfp/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	sources, err := rfpSourcesFromRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].Name != "Tender notice.txt" || sources[0].Text == "" {
		t.Fatalf("unexpected sources: %+v", sources)
	}
}

func TestRFPSourcesRejectsUnsupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]interface{}{
		"documents": []map[string]string{{"filename": "document.exe", "document": base64.StdEncoding.EncodeToString([]byte("MZ......"))}},
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/rfp/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if _, err := rfpSourcesFromRequest(c); err == nil {
		t.Fatal("expected an error for .exe")
	}
}
