package adapters

import (
	"net/http"
	"strings"

	"krampus/internal/call/domain"

	"github.com/gin-gonic/gin"
)

// ICEConfig holds the STUN/TURN configuration surfaced to clients.
// Values are read from pkg/config.CallConfig and injected at construction time.
type ICEConfig struct {
	StunServers []string
	TurnURL     string
	TurnUser    string
	TurnPass    string
}

// ICEHandler handles GET /api/v1/chat/call/ice-servers.
// Returns the ICE server list so the frontend never hard-codes STUN/TURN URLs.
type ICEHandler struct {
	cfg ICEConfig
}

func NewICEHandler(cfg ICEConfig) *ICEHandler {
	return &ICEHandler{cfg: cfg}
}

func (h *ICEHandler) Handle(c *gin.Context) {
	servers := make([]domain.ICEServer, 0)

	for _, url := range h.cfg.StunServers {
		url = strings.TrimSpace(url)
		if url != "" {
			servers = append(servers, domain.ICEServer{URLs: []string{url}})
		}
	}

	if h.cfg.TurnURL != "" {
		servers = append(servers, domain.ICEServer{
			URLs:       []string{h.cfg.TurnURL},
			Username:   h.cfg.TurnUser,
			Credential: h.cfg.TurnPass,
		})
	}

	c.JSON(http.StatusOK, domain.ICEServersResponse{ICEServers: servers})
}
