package hub

import "sync"

// Hub manages SSE subscriber channels keyed by chat ID.
// Keys use the format "private:<uuid>" or "group:<uuid>".
type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan []byte
}

func New() *Hub {
	return &Hub{subs: make(map[string][]chan []byte)}
}

// Subscribe registers a new channel for the given key and returns it.
func (h *Hub) Subscribe(key string) chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.subs[key] = append(h.subs[key], ch)
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the key's subscriber list.
func (h *Hub) Unsubscribe(key string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.subs[key]
	for i, c := range list {
		if c == ch {
			h.subs[key] = append(list[:i], list[i+1:]...)
			close(ch)
			return
		}
	}
}

// Broadcast sends data to every subscriber of key (non-blocking; slow clients are skipped).
func (h *Hub) Broadcast(key string, data []byte) {
	h.mu.Lock()
	list := h.subs[key]
	h.mu.Unlock()
	for _, ch := range list {
		select {
		case ch <- data:
		default:
		}
	}
}
