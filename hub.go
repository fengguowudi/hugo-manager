package main

import "sync"

const maxHistory = 300

type Hub struct {
	mu      sync.Mutex
	history []string
}

func newHub() *Hub { return &Hub{} }

func (h *Hub) log(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history = append(h.history, msg)
	if len(h.history) > maxHistory {
		h.history = h.history[len(h.history)-maxHistory:]
	}
}

func (h *Hub) state() {}

func (h *Hub) historySnapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.history))
	copy(out, h.history)
	return out
}
