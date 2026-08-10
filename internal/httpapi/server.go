package httpapi

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/artifacts"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/audit"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/cluster"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/discord"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/oci"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/oidc"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/orgs"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/packages"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/pipeline"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/notifications"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/registry"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/releases"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/runners"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	AllowRegister  bool
	SessionTTL     time.Duration
	NodeID         string
	SecretsKey     string
	WebhookSecret  string
	PublicURL      string
	APIURL         string
	OIDC           []oidc.ProviderConfig
}

type Server struct {
	pool           *pgxpool.Pool
	store          storage.Store
	identity       *identity.Service
	orgs           *orgs.Service
	audit          *audit.Logger
	pipelines      *pipeline.Service
	runners        *runners.Service
	artifacts      *artifacts.Service
	releases       *releases.Service
	packages       *packages.Service
	registry       *registry.Service
	cluster        *cluster.Service
	oci            *oci.Distribution
	secrets        *secrets.Box
	oidc           *oidc.Service
	scm            *scm.Service
	notifications  *notifications.Service
	discord        *discord.Service
	opts           Options
	started        time.Time
	mux            *http.ServeMux
}

func New(pool *pgxpool.Pool, store storage.Store, opts Options) *Server {
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 7 * 24 * time.Hour
	}
	reg := registry.New(pool, store)
	s := &Server{
		pool:      pool,
		store:     store,
		identity:  identity.New(pool),
		orgs:      orgs.New(pool),
		audit:     audit.New(pool),
		pipelines: pipeline.NewService(pool),
		runners:   runners.New(pool),
		artifacts: artifacts.New(pool, store),
		releases:  releases.New(pool),
		packages:  packages.New(pool, store),
		registry:  reg,
		cluster:   cluster.New(pool, opts.NodeID),
		oci:           oci.NewDistribution(pool, store, reg),
		oidc:          oidc.New(opts.OIDC),
		scm:           scm.New(pool),
		notifications: notifications.New(pool),
		discord:       discord.New(pool, opts.PublicURL),
		opts:          opts,
		started:   time.Now().UTC(),
		mux:       http.NewServeMux(),
	}
	if opts.SecretsKey != "" {
		if box, err := secrets.New(pool, opts.SecretsKey); err == nil {
			s.secrets = box
		}
	}
	s.routes()
	go s.background()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.withCORS(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /api/v1/system/info", s.handleSystemInfo)

	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/me", s.requireAuth(s.handleMe))
	s.mux.HandleFunc("POST /api/v1/me/tokens", s.requireAuth(s.handleCreateToken))
	s.mux.HandleFunc("GET /api/v1/auth/oidc/providers", s.handleListOIDCProviders)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/{provider}/start", s.handleOIDCStart)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/{provider}/callback", s.handleOIDCCallback)

	s.mux.HandleFunc("GET /api/v1/orgs", s.requireAuth(s.handleListOrgs))
	s.mux.HandleFunc("POST /api/v1/orgs", s.requireAuth(s.handleCreateOrg))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}", s.requireAuth(s.handleGetOrg))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/members", s.requireAuth(s.handleListMembers))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/members", s.requireAuth(s.handleAddMember))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects", s.requireAuth(s.handleListProjects))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects", s.requireAuth(s.handleCreateProject))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}", s.requireAuth(s.handleGetProject))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/pipelines", s.requireAuth(s.handleListPipelines))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/pipelines", s.requireAuth(s.handleUpsertPipeline))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/pipelines/{pipelineID}/runs", s.requireAuth(s.handleStartRun))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/runs", s.requireAuth(s.handleListRuns))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/runs/{runID}", s.requireAuth(s.handleGetRun))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/runs/{runID}/cancel", s.requireAuth(s.handleCancelRun))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/runs/{runID}/jobs", s.requireAuth(s.handleListRunJobs))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/jobs/{jobID}/logs", s.requireAuth(s.handleJobLogs))

	s.mux.HandleFunc("GET /api/v1/runners", s.requireAuth(s.handleListRunners))
	s.mux.HandleFunc("POST /api/v1/runners/registration-tokens", s.requireAuth(s.handleCreateRunnerRegToken))
	s.mux.HandleFunc("POST /api/v1/runners/install-token", s.requireAuth(s.handleCreateRunnerInstall))
	s.mux.HandleFunc("GET /api/v1/runners/install.sh", s.handleRunnerInstallScript)
	s.mux.HandleFunc("POST /api/v1/runner/register", s.handleRunnerRegister)
	s.mux.HandleFunc("POST /api/v1/runner/heartbeat", s.requireRunner(s.handleRunnerHeartbeat))
	s.mux.HandleFunc("POST /api/v1/runner/jobs/lease", s.requireRunner(s.handleRunnerLease))
	s.mux.HandleFunc("POST /api/v1/runner/jobs/{jobID}/start", s.requireRunner(s.handleRunnerStartJob))
	s.mux.HandleFunc("POST /api/v1/runner/jobs/{jobID}/complete", s.requireRunner(s.handleRunnerCompleteJob))
	s.mux.HandleFunc("POST /api/v1/runner/jobs/{jobID}/logs", s.requireRunner(s.handleRunnerAppendLog))
	s.mux.HandleFunc("POST /api/v1/runner/steps/{stepID}/status", s.requireRunner(s.handleRunnerStepStatus))
	s.mux.HandleFunc("GET /api/v1/runner/jobs/{jobID}/steps", s.requireRunner(s.handleRunnerJobSteps))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/artifacts", s.requireAuth(s.handleListArtifacts))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/artifacts", s.requireAuth(s.handleUploadArtifact))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/artifacts/{artifactID}/download", s.requireAuth(s.handleDownloadArtifact))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/environments", s.requireAuth(s.handleListEnvironments))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/environments", s.requireAuth(s.handleCreateEnvironment))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/releases", s.requireAuth(s.handleListReleases))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/releases", s.requireAuth(s.handleCreateRelease))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/deployments", s.requireAuth(s.handleListDeployments))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/deployments", s.requireAuth(s.handleCreateDeployment))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/packages", s.requireAuth(s.handleListPackageRepos))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/packages", s.requireAuth(s.handleCreatePackageRepo))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/packages/{repoID}/versions", s.requireAuth(s.handleListPackageVersions))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/packages/{repoID}/versions", s.requireAuth(s.handlePublishPackage))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/oci", s.requireAuth(s.handleListOCIRepos))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/oci/{name}/manifests/{tag}", s.requireAuth(s.handlePutOCIManifest))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/oci/{name}/tags", s.requireAuth(s.handleListOCITags))
	s.mux.HandleFunc("POST /api/v1/webhooks/{provider}", s.handleWebhook)

	s.mux.HandleFunc("GET /api/v1/scm/providers", s.requireAuth(s.handleListSCMProviders))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/scm/connections", s.requireAuth(s.handleListSCMConnections))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects/{projectID}/scm/connections", s.requireAuth(s.handleCreateSCMConnection))
	s.mux.HandleFunc("DELETE /api/v1/orgs/{orgID}/projects/{projectID}/scm/connections/{connectionID}", s.requireAuth(s.handleDeleteSCMConnection))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects/{projectID}/webhooks/deliveries", s.requireAuth(s.handleListWebhookDeliveries))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/forge/credentials", s.requireAuth(s.handleListForgeCredentials))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/forge/credentials", s.requireAuth(s.handleCreateForgeCredential))
	s.mux.HandleFunc("DELETE /api/v1/orgs/{orgID}/forge/credentials/{credentialID}", s.requireAuth(s.handleDeleteForgeCredential))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/forge/credentials/{credentialID}/remote-orgs", s.requireAuth(s.handleListRemoteOrgs))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/forge/credentials/{credentialID}/remote-repos", s.requireAuth(s.handleListRemoteRepos))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/forge/imports", s.requireAuth(s.handleStartForgeImport))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/forge/imports/{jobID}", s.requireAuth(s.handleGetForgeImport))
	s.mux.HandleFunc("GET /api/v1/notifications", s.requireAuth(s.handleListNotifications))
	s.mux.HandleFunc("POST /api/v1/notifications/read-all", s.requireAuth(s.handleMarkAllNotificationsRead))
	s.mux.HandleFunc("POST /api/v1/notifications/{notificationID}/read", s.requireAuth(s.handleMarkNotificationRead))

	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/discord", s.requireAuth(s.handleListDiscord))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/discord", s.requireAuth(s.handleCreateDiscord))
	s.mux.HandleFunc("DELETE /api/v1/orgs/{orgID}/discord/{integrationID}", s.requireAuth(s.handleDeleteDiscord))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/discord/{integrationID}/test", s.requireAuth(s.handleTestDiscord))

	s.mux.HandleFunc("GET /api/v1/secrets", s.requireAuth(s.handleListSecrets))
	s.mux.HandleFunc("POST /api/v1/secrets", s.requireAuth(s.handleCreateSecret))

	s.mux.HandleFunc("GET /repository/npm/{org}/{project}/{repo}/{name}/-/{filename}", s.handleNPMTarball)
	s.mux.HandleFunc("PUT /repository/npm/{org}/{project}/{repo}/{name...}", s.handleNPMPublish)
	s.mux.HandleFunc("GET /repository/npm/{org}/{project}/{repo}/{name...}", s.handleNPMPackument)
	s.mux.HandleFunc("PUT /repository/maven/{org}/{project}/{repo}/{path...}", s.handleMavenPut)
	s.mux.HandleFunc("GET /repository/maven/{org}/{project}/{repo}/{path...}", s.handleMavenGet)

	s.mux.HandleFunc("GET /auth/token", s.handleRegistryToken)
	s.mux.HandleFunc("POST /auth/token", s.handleRegistryToken)
	s.mux.HandleFunc("HEAD /v2/{rest...}", s.requireRegistryAuth(s.handleOCIDistribution))
	s.mux.HandleFunc("GET /v2/{rest...}", s.handleOCIGet)
	s.mux.HandleFunc("POST /v2/{rest...}", s.requireRegistryAuth(s.handleOCIDistribution))
	s.mux.HandleFunc("PUT /v2/{rest...}", s.requireRegistryAuth(s.handleOCIDistribution))
}

func (s *Server) background() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		_ = s.cluster.Heartbeat(ctx)
		if ok, _, err := s.cluster.AcquireLease(ctx, "scheduler", 15*time.Second); err == nil && ok {
			_, _ = s.runners.ExpireLeases(ctx)
			rows, err := s.pool.Query(ctx, `SELECT id FROM pipeline_runs WHERE status IN ('queued','running')`)
			if err == nil {
				for rows.Next() {
					var id string
					if rows.Scan(&id) == nil {
						_ = s.pipelines.AdvanceRunGraph(ctx, id)
					}
				}
				rows.Close()
			}
		}
		cancel()
	}
}

func (s *Server) projectAccess(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (orgs.Organization, orgs.Project, bool) {
	orgID := r.PathValue("orgID")
	projectID := r.PathValue("projectID")
	org, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, perm)
	if err != nil {
		mapIdentityError(w, err)
		return orgs.Organization{}, orgs.Project{}, false
	}
	project, err := s.orgs.GetProject(r.Context(), org.ID, projectID)
	if err != nil {
		mapIdentityError(w, err)
		return orgs.Organization{}, orgs.Project{}, false
	}
	return org, project, true
}

func (s *Server) orgAccess(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (orgs.Organization, bool) {
	orgID := r.PathValue("orgID")
	org, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, perm)
	if err != nil {
		mapIdentityError(w, err)
		return orgs.Organization{}, false
	}
	return org, true
}

func readBodyLimited(r *http.Request, max int64) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, max))
}

func queryInt64(r *http.Request, key string, fallback int64) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
