package scm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/orgs"
)

type ImportJob struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	CredentialID   string    `json:"credential_id"`
	RemoteOrg      string    `json:"remote_org"`
	Status         string    `json:"status"`
	TotalCount     int       `json:"total_count"`
	CompletedCount int       `json:"completed_count"`
	CreatedCount   int       `json:"created_count"`
	SkippedCount   int       `json:"skipped_count"`
	FailedCount    int       `json:"failed_count"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ImportJobItem struct {
	ID           string `json:"id"`
	JobID        string `json:"job_id"`
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
	Status       string `json:"status"`
	ProjectID    string `json:"project_id,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type StartImportInput struct {
	OrganizationID string
	CredentialID   string
	RemoteOrg      string
	Repos          []RemoteRepo
	PublicBaseURL  string
	ActorID        string
	Orgs           *orgs.Service
}

var slugCleaner = regexp.MustCompile(`[^a-z0-9-]+`)

func ProjectSlugFromRepo(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = slugCleaner.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "repo"
	}
	if len(s) > 63 {
		s = s[:63]
		s = strings.Trim(s, "-")
	}
	return s
}

func (s *Service) FindConnectionByRepo(ctx context.Context, orgID, provider, owner, repo string) (Connection, error) {
	var c Connection
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
		       bot_username, pipeline_slug, enabled, created_at, updated_at,
		       access_token <> '', webhook_secret <> '', COALESCE(created_by::text,'')
		FROM scm_connections
		WHERE organization_id = $1 AND provider = $2
		  AND lower(repo_owner) = lower($3) AND lower(repo_name) = lower($4)
		LIMIT 1
	`, orgID, NormalizeProvider(provider), owner, repo).Scan(
		&c.ID, &c.OrganizationID, &c.ProjectID, &c.Provider, &c.Name, &c.BaseURL, &c.RepoOwner, &c.RepoName,
		&c.BotUsername, &c.PipelineSlug, &c.Enabled, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.HasSecret, &c.CreatedBy,
	)
	if err != nil {
		return Connection{}, identity.ErrNotFound
	}
	return c, nil
}

func (s *Service) StartImport(ctx context.Context, in StartImportInput) (ImportJob, error) {
	if in.Orgs == nil {
		return ImportJob{}, fmt.Errorf("%w: orgs service required", identity.ErrInvalidInput)
	}
	if len(in.Repos) == 0 {
		return ImportJob{}, fmt.Errorf("%w: repos required", identity.ErrInvalidInput)
	}
	cred, err := s.GetCredential(ctx, in.OrganizationID, in.CredentialID)
	if err != nil {
		return ImportJob{}, err
	}
	var job ImportJob
	err = s.pool.QueryRow(ctx, `
		INSERT INTO forge_import_jobs (organization_id, credential_id, remote_org, status, total_count, created_by)
		VALUES ($1,$2,$3,'queued',$4,$5)
		RETURNING id, organization_id, credential_id, remote_org, status, total_count, completed_count,
		          created_count, skipped_count, failed_count, error_message, created_at, updated_at,
		          COALESCE(created_by::text,'')
	`, in.OrganizationID, in.CredentialID, strings.TrimSpace(in.RemoteOrg), len(in.Repos), nullIfEmpty(in.ActorID),
	).Scan(
		&job.ID, &job.OrganizationID, &job.CredentialID, &job.RemoteOrg, &job.Status, &job.TotalCount, &job.CompletedCount,
		&job.CreatedCount, &job.SkippedCount, &job.FailedCount, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.CreatedBy,
	)
	if err != nil {
		return ImportJob{}, err
	}

	itemIDs := make([]string, 0, len(in.Repos))
	for _, repo := range in.Repos {
		var itemID string
		err := s.pool.QueryRow(ctx, `
			INSERT INTO forge_import_job_items (job_id, repo_owner, repo_name, status)
			VALUES ($1,$2,$3,'queued') RETURNING id
		`, job.ID, repo.Owner, repo.Name).Scan(&itemID)
		if err != nil {
			return ImportJob{}, err
		}
		itemIDs = append(itemIDs, itemID)
	}

	go s.runImportJob(job.ID, cred, in)

	_ = itemIDs
	return job, nil
}

func (s *Service) GetImportJob(ctx context.Context, orgID, jobID string) (ImportJob, []ImportJobItem, error) {
	var job ImportJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, credential_id, remote_org, status, total_count, completed_count,
		       created_count, skipped_count, failed_count, error_message, created_at, updated_at,
		       COALESCE(created_by::text,'')
		FROM forge_import_jobs WHERE organization_id = $1 AND id = $2
	`, orgID, jobID).Scan(
		&job.ID, &job.OrganizationID, &job.CredentialID, &job.RemoteOrg, &job.Status, &job.TotalCount, &job.CompletedCount,
		&job.CreatedCount, &job.SkippedCount, &job.FailedCount, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.CreatedBy,
	)
	if err != nil {
		return ImportJob{}, nil, identity.ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, repo_owner, repo_name, status,
		       COALESCE(project_id::text,''), COALESCE(connection_id::text,''), error_message
		FROM forge_import_job_items WHERE job_id = $1 ORDER BY repo_name
	`, jobID)
	if err != nil {
		return job, nil, err
	}
	defer rows.Close()
	var items []ImportJobItem
	for rows.Next() {
		var it ImportJobItem
		if err := rows.Scan(&it.ID, &it.JobID, &it.RepoOwner, &it.RepoName, &it.Status, &it.ProjectID, &it.ConnectionID, &it.ErrorMessage); err != nil {
			return job, nil, err
		}
		items = append(items, it)
	}
	return job, items, rows.Err()
}

func (s *Service) runImportJob(jobID string, cred ForgeCredential, in StartImportInput) {
	ctx := context.Background()
	_, _ = s.pool.Exec(ctx, `UPDATE forge_import_jobs SET status='running', updated_at=now() WHERE id=$1`, jobID)

	type work struct {
		itemID string
		repo   RemoteRepo
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, repo_owner, repo_name FROM forge_import_job_items WHERE job_id = $1 AND status = 'queued'
	`, jobID)
	if err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE forge_import_jobs SET status='failed', error_message=$2, updated_at=now() WHERE id=$1`, jobID, err.Error())
		return
	}
	var works []work
	for rows.Next() {
		var w work
		if err := rows.Scan(&w.itemID, &w.repo.Owner, &w.repo.Name); err != nil {
			rows.Close()
			_, _ = s.pool.Exec(ctx, `UPDATE forge_import_jobs SET status='failed', error_message=$2, updated_at=now() WHERE id=$1`, jobID, err.Error())
			return
		}
		works = append(works, w)
	}
	rows.Close()

	var created, skipped, failed, completed atomic.Int64
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, w := range works {
		wg.Add(1)
		sem <- struct{}{}
		go func(w work) {
			defer wg.Done()
			defer func() { <-sem }()
			status, projectID, connID, errMsg := s.importOneRepo(ctx, cred, in, w.repo)
			_, _ = s.pool.Exec(ctx, `
				UPDATE forge_import_job_items
				SET status=$2, project_id=NULLIF($3,'')::uuid, connection_id=NULLIF($4,'')::uuid,
				    error_message=$5, updated_at=now()
				WHERE id=$1
			`, w.itemID, status, projectID, connID, errMsg)
			completed.Add(1)
			switch status {
			case "created":
				created.Add(1)
			case "skipped_exists":
				skipped.Add(1)
			default:
				failed.Add(1)
			}
			_, _ = s.pool.Exec(ctx, `
				UPDATE forge_import_jobs SET
					completed_count=$2, created_count=$3, skipped_count=$4, failed_count=$5, updated_at=now()
				WHERE id=$1
			`, jobID, completed.Load(), created.Load(), skipped.Load(), failed.Load())
		}(w)
	}
	wg.Wait()
	_, _ = s.pool.Exec(ctx, `UPDATE forge_import_jobs SET status='completed', updated_at=now() WHERE id=$1`, jobID)
}

func (s *Service) importOneRepo(ctx context.Context, cred ForgeCredential, in StartImportInput, repo RemoteRepo) (status, projectID, connID, errMsg string) {
	if existing, err := s.FindConnectionByRepo(ctx, in.OrganizationID, cred.Provider, repo.Owner, repo.Name); err == nil {
		return "skipped_exists", existing.ProjectID, existing.ID, ""
	}

	slug := ProjectSlugFromRepo(repo.Name)
	name := repo.Name
	desc := repo.Description
	project, err := in.Orgs.CreateProject(ctx, in.ActorID, in.OrganizationID, slug, name, desc)
	if err != nil {
		for i := 2; i <= 20; i++ {
			alt := fmt.Sprintf("%s-%d", slug, i)
			if len(alt) > 63 {
				alt = alt[:63]
			}
			project, err = in.Orgs.CreateProject(ctx, in.ActorID, in.OrganizationID, alt, name, desc)
			if err == nil {
				break
			}
		}
		if err != nil {
			return "failed", "", "", err.Error()
		}
	}

	secret, _ := randomSecret(24)
	connToken := cred.AccessToken
	if cred.InstallationID != "" {
		connToken = ""
	}
	connName := ProjectSlugFromRepo(repo.Owner + "-" + repo.Name)
	conn, err := s.Create(ctx, CreateInput{
		OrganizationID: in.OrganizationID,
		ProjectID:      project.ID,
		Provider:       cred.Provider,
		Name:           connName,
		BaseURL:        cred.BaseURL,
		RepoOwner:      repo.Owner,
		RepoName:       repo.Name,
		AccessToken:    connToken,
		InstallationID: cred.InstallationID,
		WebhookSecret:  secret,
		ActorID:        in.ActorID,
	})
	if err != nil {
		return "failed", project.ID, "", err.Error()
	}

	if in.PublicBaseURL != "" {
		hookURL := strings.TrimRight(in.PublicBaseURL, "/") + fmt.Sprintf("/api/v1/webhooks/%s?connection_id=%s", conn.Provider, conn.ID)
		if err := RegisterWebhook(ctx, cred, repo.Owner, repo.Name, hookURL, secret); err != nil {
			return "created", project.ID, conn.ID, explainHookError(err, hookURL)
		}
	}
	return "created", project.ID, conn.ID, ""
}

func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func explainHookError(err error, hookURL string) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "isn't reachable over the public Internet"), strings.Contains(msg, "not supported because"):
		return "webhook skipped: the forge cannot reach " + hookURL + ". Set a public URL in Settings, then re-register the hook."
	case strings.Contains(msg, "404"), strings.Contains(msg, "Not Found"):
		return "webhook skipped: no permission to create hooks on this repo. Grant the app read & write access to repository webhooks."
	case strings.Contains(msg, "403"):
		return "webhook skipped: the forge refused hook creation (403). Check the token or app permissions for webhooks."
	default:
		return "webhook skipped: " + msg
	}
}
