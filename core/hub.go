package core

import "sync"

const maxHistory = 300

type Hub struct {
	mu      sync.Mutex
	history []string
}

func NewHub() *Hub { return &Hub{} }

func (h *Hub) Log(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history = append(h.history, msg)
	if len(h.history) > maxHistory {
		h.history = h.history[len(h.history)-maxHistory:]
	}
}

func (h *Hub) State() {}

func (h *Hub) HistorySnapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.history))
	copy(out, h.history)
	return out
}
