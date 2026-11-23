package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Gin panics at startup on conflicting routes; mirror the chat/call routes from cmd/server.
func TestChatAndGroupCallRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noop := func(c *gin.Context) { c.String(http.StatusOK, c.FullPath()) }
	api := r.Group("/api")
	api.GET("/chats", noop)
	api.GET("/chats/calls/incoming", noop)
	api.GET("/chats/calls/:call_id", noop)
	api.POST("/chats/calls/:call_id/accept", noop)
	api.GET("/chats/calls/:call_id/candidates", noop)
	api.GET("/chats/:user_id/messages", noop)
	api.GET("/chats/:user_id/typing", noop)
	api.POST("/chats/:user_id/typing", noop)
	api.GET("/profile", noop)
	api.POST("/profile/photo", noop)
	api.GET("/tasks", noop)
	api.GET("/tasks/summary", noop)
	api.PATCH("/tasks/:task_id", noop)
	api.POST("/chats/:user_id/messages", noop)
	api.POST("/chats/:user_id/calls", noop)
	api.GET("/calls/ice-servers", noop)
	api.POST("/chats/messages/:message_id/reactions", noop)
	g := api.Group("/group-calls")
	g.POST("", noop)
	g.GET("/incoming", noop)
	g.GET("/:call_id/sync", noop)
	g.POST("/:call_id/join", noop)
	g.POST("/:call_id/signals", noop)

	cases := map[string]string{
		"POST /api/chats/messages/12/reactions": "/api/chats/messages/:message_id/reactions",
		"GET /api/chats/7/messages":             "/api/chats/:user_id/messages",
		"GET /api/group-calls/incoming":         "/api/group-calls/incoming",
		"GET /api/chats/7/typing":               "/api/chats/:user_id/typing",
		"GET /api/tasks/summary":                "/api/tasks/summary",
		"PATCH /api/tasks/3":                    "/api/tasks/:task_id",
		"GET /api/group-calls/abc/sync":         "/api/group-calls/:call_id/sync",
	}
	for req, want := range cases {
		var method, path string
		for i := range req {
			if req[i] == ' ' {
				method, path = req[:i], req[i+1:]
				break
			}
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Body.String() != want {
			t.Errorf("%s routed to %q, want %q", req, w.Body.String(), want)
		}
	}
}
