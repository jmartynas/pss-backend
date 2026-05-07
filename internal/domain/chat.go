package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PrivateChat struct {
	ID          uuid.UUID
	OtherUserID uuid.UUID
	OtherName   string
	RouteID     uuid.UUID
	LastMessage string
	CreatedAt   time.Time
}

type GroupChat struct {
	RouteID   uuid.UUID
	RouteName string
	CreatedAt time.Time
}

type ChatMessage struct {
	ID           uuid.UUID
	SenderUserID uuid.UUID
	SenderName   string
	Message      string
	CreatedAt    time.Time
}

type ChatRepository interface {
	ListPrivateChats(ctx context.Context, userID uuid.UUID) ([]PrivateChat, error)
	ListGroupChats(ctx context.Context, userID uuid.UUID) ([]GroupChat, error)
	GetPrivateMessages(ctx context.Context, chatID uuid.UUID) ([]ChatMessage, error)
	GetGroupMessages(ctx context.Context, routeID uuid.UUID) ([]ChatMessage, error)
	SendPrivateMessage(ctx context.Context, chatID, senderUserID uuid.UUID, message string) (uuid.UUID, error)
	SendGroupMessage(ctx context.Context, routeID, senderUserID uuid.UUID, message string) (uuid.UUID, error)
	CanAccessPrivateChat(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	CanAccessGroupChat(ctx context.Context, routeID, userID uuid.UUID) (bool, error)
}
