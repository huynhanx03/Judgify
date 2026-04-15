package store

import (
	"encoding/json"
	"sync"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
)

// LeaderboardHub manages SSE subscribers per contest.
// Thread-safe: all methods can be called concurrently.
type LeaderboardHub struct {
	mu          sync.RWMutex
	subscribers map[int]map[chan json.RawMessage]struct{}
	logger      *zap.Logger
}

// NewLeaderboardHub creates a new hub.
func NewLeaderboardHub() *LeaderboardHub {
	return &LeaderboardHub{
		subscribers: make(map[int]map[chan json.RawMessage]struct{}),
		logger:      global.LoggerZap.Named("leaderboard-hub"),
	}
}

// Subscribe registers a channel for contest updates. Returns the channel.
func (h *LeaderboardHub) Subscribe(contestID int) chan json.RawMessage {
	ch := make(chan json.RawMessage, 16)

	h.mu.Lock()
	if h.subscribers[contestID] == nil {
		h.subscribers[contestID] = make(map[chan json.RawMessage]struct{})
	}
	h.subscribers[contestID][ch] = struct{}{}
	h.mu.Unlock()

	h.logger.Debug("subscriber added", zap.Int("contest_id", contestID))
	return ch
}

// Unsubscribe removes a channel and closes it.
func (h *LeaderboardHub) Unsubscribe(contestID int, ch chan json.RawMessage) {
	h.mu.Lock()
	if subs, ok := h.subscribers[contestID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subscribers, contestID)
		}
	}
	h.mu.Unlock()

	// Drain and close
	for {
		select {
		case <-ch:
		default:
			close(ch)
			return
		}
	}
}

// Broadcast sends data to all subscribers of a contest.
// Non-blocking: skips slow consumers.
func (h *LeaderboardHub) Broadcast(contestID int, data json.RawMessage) {
	h.mu.RLock()
	subs := h.subscribers[contestID]
	h.mu.RUnlock()

	for ch := range subs {
		select {
		case ch <- data:
		default:
			// Skip slow consumer
			h.logger.Warn("slow consumer skipped", zap.Int("contest_id", contestID))
		}
	}
}

// SubscriberCount returns the number of active subscribers for a contest.
func (h *LeaderboardHub) SubscriberCount(contestID int) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[contestID])
}
