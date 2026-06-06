package adapters

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"krampus/internal/call/domain"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func doICE(cfg ICEConfig) (*httptest.ResponseRecorder, domain.ICEServersResponse) {
	h := NewICEHandler(cfg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/call/ice-servers", nil)
	h.Handle(c)

	var resp domain.ICEServersResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w, resp
}

// STUN entries are surfaced; blank/whitespace entries are dropped.
func TestICEHandler_StunOnly(t *testing.T) {
	w, resp := doICE(ICEConfig{StunServers: []string{"stun:stun.l.google.com:19302", "  ", ""}})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(resp.ICEServers) != 1 {
		t.Fatalf("expected 1 server (blanks dropped), got %d", len(resp.ICEServers))
	}
	if resp.ICEServers[0].URLs[0] != "stun:stun.l.google.com:19302" {
		t.Errorf("unexpected stun url: %v", resp.ICEServers[0].URLs)
	}
}

// A configured TURN server is appended with its credentials.
func TestICEHandler_WithTurn(t *testing.T) {
	_, resp := doICE(ICEConfig{
		StunServers: []string{"stun:stun.l.google.com:19302"},
		TurnURL:     "turn:turn.example.com:3478",
		TurnUser:    "user",
		TurnPass:    "pass",
	})

	if len(resp.ICEServers) != 2 {
		t.Fatalf("expected stun+turn, got %d", len(resp.ICEServers))
	}
	turn := resp.ICEServers[1]
	if turn.URLs[0] != "turn:turn.example.com:3478" || turn.Username != "user" || turn.Credential != "pass" {
		t.Errorf("turn entry malformed: %+v", turn)
	}
}

// No config yields an empty (non-null) iceServers array.
func TestICEHandler_Empty(t *testing.T) {
	_, resp := doICE(ICEConfig{})
	if resp.ICEServers == nil {
		t.Fatal("iceServers should serialize as [] not null")
	}
	if len(resp.ICEServers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(resp.ICEServers))
	}
}
