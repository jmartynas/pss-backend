package hub

import "sync"

type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan []byte
}

func New() *Hub {
	return &Hub{subs: make(map[string][]chan []byte)}
}

func (h *Hub) Subscribe(key string) chan []byte {
	ch := make(chan []byte, 16)

	h.mu.Lock()
	h.subs[key] = append(h.subs[key], ch)
	h.mu.Unlock()

	return ch
}

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
