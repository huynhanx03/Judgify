package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/internal/contest/store"
	handlerCommon "github.com/huynhanx03/judgify/pkg/common/http/handler"
)

// StandingHandler defines the standing HTTP handler interface.
type StandingHandler interface {
	GetStandings(ctx context.Context, req *dto.GetStandingsRequest) ([]*dto.StandingResponse, error)
}

type standingHandler struct {
	handlerCommon.BaseHandler
	standingService ports.StandingService
	hub             *store.LeaderboardHub
}

// NewStandingHandler creates a new standingHandler instance.
func NewStandingHandler(standingService ports.StandingService, hub *store.LeaderboardHub) *standingHandler {
	return &standingHandler{standingService: standingService, hub: hub}
}

func (h *standingHandler) GetStandings(ctx context.Context, req *dto.GetStandingsRequest) ([]*dto.StandingResponse, error) {
	return h.standingService.GetStandings(ctx, req.ContestID)
}

// StreamStandings is a Gin handler for SSE realtime standings.
func (h *standingHandler) StreamStandings(c *gin.Context) {
	contestIDStr := c.Param("id")
	var contestID int
	if _, err := fmt.Sscanf(contestIDStr, "%d", &contestID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contest id"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Send initial snapshot
	standings, err := h.standingService.GetStandings(c.Request.Context(), contestID)
	if err != nil {
		c.SSEvent("error", gin.H{"message": "failed to load standings"})
		return
	}
	snapshot, _ := json.Marshal(standings)
	c.SSEvent("snapshot", string(snapshot))
	c.Writer.Flush()

	// Subscribe to updates
	ch := h.hub.Subscribe(contestID)
	defer h.hub.Unsubscribe(contestID, ch)

	// Stream updates
	c.Stream(func(w io.Writer) bool {
		select {
		case data, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("update", string(data))
			return true
		case <-c.Request.Context().Done():
			return false
		case <-c.Done():
			return false
		}
	})
}
