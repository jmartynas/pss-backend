package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/hub"
	"github.com/jmartynas/pss-backend/internal/middleware"
)

type ChatHandler struct {
	repo domain.ChatRepository
	hub  *hub.Hub
	log  *slog.Logger
}

func NewChatHandler(repo domain.ChatRepository, h *hub.Hub, log *slog.Logger) *ChatHandler {
	return &ChatHandler{repo: repo, hub: h, log: log}
}

func (h *ChatHandler) ListPrivateChats(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	chats, err := h.repo.ListPrivateChats(r.Context(), u.ID)
	if err != nil {
		h.log.Error("list private chats", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if chats == nil {
		chats = []domain.PrivateChat{}
	}
	writeJSON(w, http.StatusOK, chats)
}

func (h *ChatHandler) ListGroupChats(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	chats, err := h.repo.ListGroupChats(r.Context(), u.ID)
	if err != nil {
		h.log.Error("list group chats", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if chats == nil {
		chats = []domain.GroupChat{}
	}
	writeJSON(w, http.StatusOK, chats)
}

func (h *ChatHandler) GetPrivateMessages(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	chatID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessPrivateChat(r.Context(), chatID, u.ID)
	if err != nil {
		h.log.Error("can access private chat", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	msgs, err := h.repo.GetPrivateMessages(r.Context(), chatID)
	if err != nil {
		h.log.Error("get private messages", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []domain.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *ChatHandler) GetGroupMessages(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessGroupChat(r.Context(), routeID, u.ID)
	if err != nil {
		h.log.Error("can access group chat", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	msgs, err := h.repo.GetGroupMessages(r.Context(), routeID)
	if err != nil {
		h.log.Error("get group messages", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []domain.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *ChatHandler) SendPrivateMessage(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	chatID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessPrivateChat(r.Context(), chatID, u.ID)
	if err != nil {
		h.log.Error("can access private chat", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	id, err := h.repo.SendPrivateMessage(r.Context(), chatID, u.ID, strings.TrimSpace(in.Message))
	if err != nil {
		h.log.Error("send private message", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	msgs, err := h.repo.GetPrivateMessages(r.Context(), chatID)
	if err == nil {
		if b, err := json.Marshal(msgs); err == nil {
			h.hub.Broadcast(fmt.Sprintf("private:%s", chatID), b)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *ChatHandler) SendGroupMessage(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessGroupChat(r.Context(), routeID, u.ID)
	if err != nil {
		h.log.Error("can access group chat", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	id, err := h.repo.SendGroupMessage(r.Context(), routeID, u.ID, strings.TrimSpace(in.Message))
	if err != nil {
		h.log.Error("send group message", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	msgs, err := h.repo.GetGroupMessages(r.Context(), routeID)
	if err == nil {
		if b, err := json.Marshal(msgs); err == nil {
			h.hub.Broadcast(fmt.Sprintf("group:%s", routeID), b)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *ChatHandler) StreamPrivate(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	chatID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessPrivateChat(r.Context(), chatID, u.ID)
	if err != nil || !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.streamSSE(w, r, fmt.Sprintf("private:%s", chatID))
}

func (h *ChatHandler) StreamGroup(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	ok, err := h.repo.CanAccessGroupChat(r.Context(), routeID, u.ID)
	if err != nil || !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.streamSSE(w, r, fmt.Sprintf("group:%s", routeID))
}

func (h *ChatHandler) streamSSE(w http.ResponseWriter, r *http.Request, key string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	// disable write deadline so the connection stays open indefinitely
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		h.log.Warn("SSE: could not clear write deadline", slog.Any("error", err))
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.hub.Subscribe(key)
	defer h.hub.Unsubscribe(key, ch)

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case data, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

