package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PrivateChat is a 1-on-1 chat between two route participants.
type PrivateChat struct {
	ID          uuid.UUID
	OtherUserID uuid.UUID
	OtherName   string
	RouteID     uuid.UUID
	LastMessage string
	CreatedAt   time.Time
}

// GroupChat represents a route's group chat channel.
type GroupChat struct {
	RouteID   uuid.UUID
	RouteName string // "From → To"
	CreatedAt time.Time
}

// ChatMessage is a single message in either a private or group chat.
type ChatMessage struct {
	ID           uuid.UUID
	SenderUserID uuid.UUID
	SenderName   string
	Message      string
	CreatedAt    time.Time
}

// ChatRepository is the persistence contract for chats and messages.
type ChatRepository interface {
	// ListPrivateChats returns all private chats where the user is a participant.
	ListPrivateChats(ctx context.Context, userID uuid.UUID) ([]PrivateChat, error)
	// ListGroupChats returns all routes the user participates in as group chats.
	ListGroupChats(ctx context.Context, userID uuid.UUID) ([]GroupChat, error)
	// GetPrivateMessages returns messages for a private chat, newest last.
	GetPrivateMessages(ctx context.Context, chatID uuid.UUID) ([]ChatMessage, error)
	// GetGroupMessages returns messages for a route's group chat, newest last.
	GetGroupMessages(ctx context.Context, routeID uuid.UUID) ([]ChatMessage, error)
	// SendPrivateMessage inserts a message into a private chat.
	SendPrivateMessage(ctx context.Context, chatID, senderUserID uuid.UUID, message string) (uuid.UUID, error)
	// SendGroupMessage inserts a message into a route's group chat.
	SendGroupMessage(ctx context.Context, routeID, senderUserID uuid.UUID, message string) (uuid.UUID, error)
	// CanAccessPrivateChat checks the user is a participant of the chat.
	CanAccessPrivateChat(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	// CanAccessGroupChat checks the user is a driver or approved participant of the route.
	CanAccessGroupChat(ctx context.Context, routeID, userID uuid.UUID) (bool, error)
}
