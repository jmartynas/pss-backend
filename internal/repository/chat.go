package repository

import (
	"context"
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
)

type chatRepository struct{ db *sql.DB }

func NewChatRepository(db *sql.DB) domain.ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) ListPrivateChats(ctx context.Context, userID uuid.UUID) ([]domain.PrivateChat, error) {
	// Find all chats where the user is either user1 or user2 (via participant → user).
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			pc.id,
			pc.created_at,
			p_other.user_id,
			COALESCE(u_other.name, u_other.email, ''),
			p_self.route_id
		FROM private_chats pc
		JOIN participants p_self  ON (p_self.id  = pc.user1_id OR p_self.id  = pc.user2_id)
		JOIN participants p_other ON (p_other.id = pc.user1_id OR p_other.id = pc.user2_id)
		JOIN users u_other ON u_other.id = p_other.user_id
		WHERE p_self.user_id  = ?
		  AND p_other.user_id != ?
	`, userID.String(), userID.String())
	if err != nil {
		return nil, fmt.Errorf("list private chats: %w", err)
	}
	defer rows.Close()

	var out []domain.PrivateChat
	for rows.Next() {
		var c domain.PrivateChat
		var idStr, otherUserIDStr, routeIDStr string
		if err := rows.Scan(&idStr, &c.CreatedAt, &otherUserIDStr, &c.OtherName, &routeIDStr); err != nil {
			return nil, fmt.Errorf("list private chats scan: %w", err)
		}
		c.ID, _ = uuid.Parse(idStr)
		c.OtherUserID, _ = uuid.Parse(otherUserIDStr)
		c.RouteID, _ = uuid.Parse(routeIDStr)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *chatRepository) ListGroupChats(ctx context.Context, userID uuid.UUID) ([]domain.GroupChat, error) {
	rows, err := sq.Select(
		"r.id",
		"COALESCE(r.start_formatted_address, CONCAT(r.start_lat, ', ', r.start_lng))",
		"COALESCE(r.end_formatted_address, CONCAT(r.end_lat, ', ', r.end_lng))",
		"r.created_at",
	).
		From("routes r").
		Join("participants p ON p.route_id = r.id").
		Where(sq.Eq{"p.user_id": userID.String(), "r.deleted_at": nil}).
		Where(sq.Expr("p.status IN ('driver','approved')")).
		Where(sq.Eq{"p.deleted_at": nil}).
		OrderBy("r.leaving_at DESC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list group chats: %w", err)
	}
	defer rows.Close()

	var out []domain.GroupChat
	for rows.Next() {
		var c domain.GroupChat
		var idStr, from, to string
		if err := rows.Scan(&idStr, &from, &to, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("list group chats scan: %w", err)
		}
		c.RouteID, _ = uuid.Parse(idStr)
		c.RouteName = from + " → " + to
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *chatRepository) GetPrivateMessages(ctx context.Context, chatID uuid.UUID) ([]domain.ChatMessage, error) {
	rows, err := sq.Select("pm.id", "pm.sender_user_id", "COALESCE(u.name, u.email, '')", "pm.message", "pm.created_at").
		From("private_messages pm").
		Join("users u ON u.id = pm.sender_user_id").
		Where(sq.Eq{"pm.chat_id": chatID.String()}).
		OrderBy("pm.created_at ASC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get private messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (r *chatRepository) GetGroupMessages(ctx context.Context, routeID uuid.UUID) ([]domain.ChatMessage, error) {
	rows, err := sq.Select("rm.id", "rm.sender_user_id", "COALESCE(u.name, u.email, '')", "rm.message", "rm.created_at").
		From("route_messages rm").
		Join("users u ON u.id = rm.sender_user_id").
		Where(sq.Eq{"rm.route_id": routeID.String()}).
		OrderBy("rm.created_at ASC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get group messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (r *chatRepository) SendPrivateMessage(ctx context.Context, chatID, senderUserID uuid.UUID, message string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := sq.Insert("private_messages").
		Columns("id", "chat_id", "sender_user_id", "message").
		Values(id.String(), chatID.String(), senderUserID.String(), message).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("send private message: %w", err)
	}
	return id, nil
}

func (r *chatRepository) SendGroupMessage(ctx context.Context, routeID, senderUserID uuid.UUID, message string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := sq.Insert("route_messages").
		Columns("id", "route_id", "sender_user_id", "message").
		Values(id.String(), routeID.String(), senderUserID.String(), message).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("send group message: %w", err)
	}
	return id, nil
}

func (r *chatRepository) CanAccessPrivateChat(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM private_chats pc
		JOIN participants p ON (p.id = pc.user1_id OR p.id = pc.user2_id)
		WHERE pc.id = ? AND p.user_id = ?
	`, chatID.String(), userID.String()).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("can access private chat: %w", err)
	}
	return count > 0, nil
}

func (r *chatRepository) CanAccessGroupChat(ctx context.Context, routeID, userID uuid.UUID) (bool, error) {
	var count int
	err := sq.Select("COUNT(*)").
		From("participants").
		Where(sq.Eq{"route_id": routeID.String(), "user_id": userID.String(), "deleted_at": nil}).
		Where(sq.Expr("status IN ('driver','approved')")).
		RunWith(r.db).QueryRowContext(ctx).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("can access group chat: %w", err)
	}
	return count > 0, nil
}

func scanMessages(rows *sql.Rows) ([]domain.ChatMessage, error) {
	var out []domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		var idStr, senderIDStr string
		if err := rows.Scan(&idStr, &senderIDStr, &m.SenderName, &m.Message, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.ID, _ = uuid.Parse(idStr)
		m.SenderUserID, _ = uuid.Parse(senderIDStr)
		out = append(out, m)
	}
	return out, rows.Err()
}
