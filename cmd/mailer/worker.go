package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	sq "github.com/Masterminds/squirrel"
	mailjet "github.com/mailjet/mailjet-apiv3-go/v4"
)

type worker struct {
	db        *sql.DB
	mj        *mailjet.Client
	fromEmail string
	fromName  string
	log       *slog.Logger
	trigger   chan struct{}
}

func (w *worker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.trigger:
			w.processBatch(ctx)
		}
	}
}

func (w *worker) processBatch(ctx context.Context) {
	rows, err := sq.Select("id", "type", "request_id").
		From("email_logs").
		Where(sq.And{sq.Eq{"status": "created"}, sq.Eq{"sent_at": nil}}).
		OrderBy("created_at ASC").
		Limit(100).
		RunWith(w.db).QueryContext(ctx)
	if err != nil {
		w.log.Error("query email_logs", slog.Any("error", err))
		return
	}
	defer rows.Close()

	var logs []emailLog
	for rows.Next() {
		var el emailLog
		if err := rows.Scan(&el.id, &el.emailType, &el.requestID); err != nil {
			w.log.Error("scan email_log", slog.Any("error", err))
			return
		}
		logs = append(logs, el)
	}
	if err := rows.Err(); err != nil {
		w.log.Error("read email_logs", slog.Any("error", err))
		return
	}

	for _, el := range logs {
		w.processOne(ctx, el)
	}
}

func (w *worker) processOne(ctx context.Context, el emailLog) {
	recipients, err := w.fetchRecipients(ctx, el.requestID, el.emailType)
	if err != nil {
		w.log.Error("fetch recipients", slog.String("email_log_id", el.id), slog.Any("error", err))
		return
	}
	if len(recipients) == 0 {
		w.markSent(ctx, el.id)
		return
	}

	subject, body := emailContent(el.emailType)

	var messages mailjet.MessagesV31
	for _, r := range recipients {
		messages.Info = append(messages.Info, mailjet.InfoMessagesV31{
			From:     &mailjet.RecipientV31{Email: w.fromEmail, Name: w.fromName},
			To:       &mailjet.RecipientsV31{{Email: r.email, Name: r.name}},
			Subject:  subject,
			TextPart: body,
		})
	}

	if _, err := w.mj.SendMailV31(&messages); err != nil {
		w.log.Error("mailjet send", slog.String("email_log_id", el.id), slog.Any("error", err))
		return
	}

	w.markSent(ctx, el.id)
	w.log.Info("email sent",
		slog.String("email_log_id", el.id),
		slog.String("type", el.emailType),
		slog.Int("recipients", len(recipients)),
	)
}

func (w *worker) fetchRecipients(ctx context.Context, requestID, emailType string) ([]recipient, error) {
	var routeID string
	err := sq.Select("p.route_id").
		From("requests r").
		Join("participants p ON p.id = r.participant_id").
		Where(sq.Eq{"r.id": requestID}).
		RunWith(w.db).QueryRowContext(ctx).Scan(&routeID)
	if err != nil {
		return nil, fmt.Errorf("get route_id: %w", err)
	}

	qb := sq.Select("u.email", "COALESCE(u.name, u.email)").
		From("participants p").
		Join("users u ON u.id = p.user_id").
		Where(sq.Expr("p.status IN ('approved', 'driver')")).
		Where(sq.Eq{"p.route_id": routeID})
	if emailType != "route_cancelled" {
		qb = qb.Where(sq.Eq{"p.deleted_at": nil})
	}
	rows, err := qb.RunWith(w.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get recipients: %w", err)
	}
	defer rows.Close()

	var out []recipient
	for rows.Next() {
		var r recipient
		if err := rows.Scan(&r.email, &r.name); err != nil {
			return nil, fmt.Errorf("scan recipient: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (w *worker) markSent(ctx context.Context, id string) {
	if _, err := sq.Update("email_logs").
		Set("status", "sent").
		Set("sent_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		RunWith(w.db).ExecContext(ctx); err != nil {
		w.log.Error("mark email_log sent", slog.String("id", id), slog.Any("error", err))
	}
}
