package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
)

type Integration struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ProjectID      string    `json:"project_id,omitempty"`
	Name           string    `json:"name"`
	Mode           string    `json:"mode"`
	ChannelID      string    `json:"channel_id,omitempty"`
	NotifyOn       []string  `json:"notify_on"`
	Enabled        bool      `json:"enabled"`
	HasWebhook     bool      `json:"has_webhook"`
	HasBotToken    bool      `json:"has_bot_token"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	WebhookURL     string    `json:"-"`
	BotToken       string    `json:"-"`
}

type Service struct {
	pool      *pgxpool.Pool
	publicURL string
	client    *http.Client
	secrets   *secrets.Box
}

func New(pool *pgxpool.Pool, publicURL string, box *secrets.Box) *Service {
	return &Service{
		pool:      pool,
		publicURL: strings.TrimRight(publicURL, "/"),
		client:    &http.Client{Timeout: 12 * time.Second},
		secrets:   box,
	}
}

type CreateInput struct {
	OrganizationID string
	ProjectID      string
	Name           string
	Mode           string
	WebhookURL     string
	BotToken       string
	ChannelID      string
	NotifyOn       []string
	ActorID        string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Integration, error) {
	name := strings.ToLower(strings.TrimSpace(in.Name))
	if name == "" {
		return Integration{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = "webhook"
	}
	if mode != "webhook" && mode != "bot" {
		return Integration{}, fmt.Errorf("%w: mode must be webhook or bot", identity.ErrInvalidInput)
	}
	if mode == "webhook" && strings.TrimSpace(in.WebhookURL) == "" {
		return Integration{}, fmt.Errorf("%w: webhook_url required", identity.ErrInvalidInput)
	}
	if mode == "bot" && (strings.TrimSpace(in.BotToken) == "" || strings.TrimSpace(in.ChannelID) == "") {
		return Integration{}, fmt.Errorf("%w: bot_token and channel_id required", identity.ErrInvalidInput)
	}
	notifyOn := in.NotifyOn
	if len(notifyOn) == 0 {
		notifyOn = []string{"run.started", "run.succeeded", "run.failed", "run.canceled"}
	}
	sealedURL, err := s.secrets.SealString(strings.TrimSpace(in.WebhookURL))
	if err != nil {
		return Integration{}, err
	}
	sealedToken, err := s.secrets.SealString(strings.TrimSpace(in.BotToken))
	if err != nil {
		return Integration{}, err
	}
	var row Integration
	err = s.pool.QueryRow(ctx, `
		INSERT INTO discord_integrations (
			organization_id, project_id, name, mode, webhook_url, bot_token, channel_id, notify_on, created_by
		) VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, NULLIF($9,'')::uuid)
		RETURNING id, organization_id, COALESCE(project_id::text,''), name, mode, channel_id, notify_on, enabled,
		          created_at, updated_at, webhook_url <> '', bot_token <> ''
	`, in.OrganizationID, in.ProjectID, name, mode, sealedURL, sealedToken,
		strings.TrimSpace(in.ChannelID), notifyOn, in.ActorID,
	).Scan(&row.ID, &row.OrganizationID, &row.ProjectID, &row.Name, &row.Mode, &row.ChannelID, &row.NotifyOn,
		&row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.HasWebhook, &row.HasBotToken)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return Integration{}, identity.ErrConflict
	}
	return row, err
}

func (s *Service) List(ctx context.Context, orgID, projectID string) ([]Integration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, COALESCE(project_id::text,''), name, mode, channel_id, notify_on, enabled,
		       created_at, updated_at, webhook_url <> '', bot_token <> ''
		FROM discord_integrations
		WHERE organization_id = $1
		  AND ($2 = '' OR project_id IS NULL OR project_id::text = $2)
		ORDER BY name
	`, orgID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Integration
	for rows.Next() {
		var row Integration
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.ProjectID, &row.Name, &row.Mode, &row.ChannelID,
			&row.NotifyOn, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.HasWebhook, &row.HasBotToken); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Service) Delete(ctx context.Context, orgID, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM discord_integrations WHERE organization_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (s *Service) getEnabledForEvent(ctx context.Context, orgID, projectID, kind string) ([]Integration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, COALESCE(project_id::text,''), name, mode, channel_id, notify_on, enabled,
		       created_at, updated_at, webhook_url, bot_token, webhook_url <> '', bot_token <> ''
		FROM discord_integrations
		WHERE enabled
		  AND organization_id = $1
		  AND (project_id IS NULL OR project_id::text = $2)
		  AND $3 = ANY(notify_on)
	`, orgID, projectID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Integration
	for rows.Next() {
		var row Integration
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.ProjectID, &row.Name, &row.Mode, &row.ChannelID,
			&row.NotifyOn, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.WebhookURL, &row.BotToken,
			&row.HasWebhook, &row.HasBotToken); err != nil {
			return nil, err
		}
		if err := s.openIntegration(&row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type Event struct {
	Kind    string
	Title   string
	Body    string
	Href    string
	Ref     string
	SHA     string
	Status  string
	Number  int64
	Project string
}

func (s *Service) Notify(ctx context.Context, orgID, projectID string, ev Event) error {
	list, err := s.getEnabledForEvent(ctx, orgID, projectID, ev.Kind)
	if err != nil {
		return err
	}
	for _, integ := range list {
		_ = s.send(ctx, integ, ev)
	}
	return nil
}

func (s *Service) send(ctx context.Context, integ Integration, ev Event) error {
	payload := map[string]any{
		"username": "Shipyard",
		"avatar_url": "",
		"embeds": []map[string]any{s.embed(ev)},
	}
	raw, _ := json.Marshal(payload)
	switch integ.Mode {
	case "bot":
		url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", integ.ChannelID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bot "+integ.BotToken)
		req.Header.Set("Content-Type", "application/json")
		res, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if res.StatusCode >= 300 {
			return fmt.Errorf("discord bot: %s", strings.TrimSpace(string(body)))
		}
		return nil
	default:
		url := integ.WebhookURL
		if !strings.Contains(url, "?") {
			url += "?wait=false"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if res.StatusCode >= 300 {
			return fmt.Errorf("discord webhook: %s", strings.TrimSpace(string(body)))
		}
		return nil
	}
}

func (s *Service) embed(ev Event) map[string]any {
	color := 0x315EFB // shipyard blue
	switch {
	case strings.Contains(ev.Kind, "succeeded"), ev.Status == "succeeded":
		color = 0x177A49
	case strings.Contains(ev.Kind, "failed"), ev.Status == "failed":
		color = 0xB92C2C
	case strings.Contains(ev.Kind, "canceled"), ev.Status == "canceled":
		color = 0x6F7A87
	case strings.Contains(ev.Kind, "started"):
		color = 0x256FAF
	}
	url := ev.Href
	if url != "" && strings.HasPrefix(url, "/") {
		url = s.publicURL + url
	}
	fields := []map[string]any{}
	if ev.Number > 0 {
		fields = append(fields, map[string]any{"name": "Run", "value": fmt.Sprintf("#%d", ev.Number), "inline": true})
	}
	if ev.Status != "" {
		fields = append(fields, map[string]any{"name": "Status", "value": ev.Status, "inline": true})
	}
	if ev.Ref != "" {
		fields = append(fields, map[string]any{"name": "Ref", "value": "`" + ev.Ref + "`", "inline": true})
	}
	if ev.SHA != "" {
		sha := ev.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fields = append(fields, map[string]any{"name": "Commit", "value": "`" + sha + "`", "inline": true})
	}
	if ev.Project != "" {
		fields = append(fields, map[string]any{"name": "Project", "value": ev.Project, "inline": true})
	}
	embed := map[string]any{
		"title":       ev.Title,
		"description": ev.Body,
		"color":       color,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"footer":      map[string]any{"text": "Shipyard · delivery control plane"},
	}
	if url != "" {
		embed["url"] = url
	}
	if len(fields) > 0 {
		embed["fields"] = fields
	}
	return embed
}

// TestPing sends a Sentry-style test message to verify the integration.
func (s *Service) TestPing(ctx context.Context, orgID, id string) error {
	var row Integration
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, COALESCE(project_id::text,''), name, mode, channel_id, notify_on, enabled,
		       created_at, updated_at, webhook_url, bot_token, webhook_url <> '', bot_token <> ''
		FROM discord_integrations WHERE organization_id = $1 AND id = $2
	`, orgID, id).Scan(&row.ID, &row.OrganizationID, &row.ProjectID, &row.Name, &row.Mode, &row.ChannelID,
		&row.NotifyOn, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.WebhookURL, &row.BotToken,
		&row.HasWebhook, &row.HasBotToken)
	if err == pgx.ErrNoRows {
		return identity.ErrNotFound
	}
	if err == nil {
		err = s.openIntegration(&row)
	}
	if err != nil {
		return err
	}
	return s.send(ctx, row, Event{
		Kind:   "test",
		Title:  "Shipyard connected",
		Body:   "Discord integration is working. You’ll get build and deploy alerts here — like Sentry, but for delivery.",
		Href:   "/settings/notifications",
		Status: "ok",
	})
}

func (s *Service) openIntegration(row *Integration) error {
	url, err := s.secrets.OpenString(row.WebhookURL)
	if err != nil {
		return fmt.Errorf("decrypt discord %s webhook url: %w", row.Name, err)
	}
	token, err := s.secrets.OpenString(row.BotToken)
	if err != nil {
		return fmt.Errorf("decrypt discord %s bot token: %w", row.Name, err)
	}
	row.WebhookURL = url
	row.BotToken = token
	return nil
}
