import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import {
  api,
  ForgeCredential,
  ForgeImportJob,
  ForgeImportJobItem,
  ForgeOAuthProvider,
  GitHubAppStatus,
  RemoteOrg,
  RemoteRepo,
  SCMProvider,
} from "../api";
import { formatTime, runStatus } from "../lib/format";
import styles from "./ImportPage.module.css";

type Step = "credential" | "org" | "repos" | "progress";

const POLL_FAILURE_LIMIT = 5;
const POLL_TIMEOUT_MS = 10 * 60 * 1000;

export function ImportPage() {
  const navigate = useNavigate();
  const { org, user, setError, refreshProjects } = useWorkspace();
  const [step, setStep] = useState<Step>("credential");
  const [providers, setProviders] = useState<SCMProvider[]>([]);
  const [oauthProviders, setOauthProviders] = useState<ForgeOAuthProvider[]>([]);
  const [githubApp, setGithubApp] = useState<GitHubAppStatus | null>(null);
  const [pendingInstall, setPendingInstall] = useState("");
  const [credentials, setCredentials] = useState<ForgeCredential[]>([]);
  const [credentialID, setCredentialID] = useState("");
  const [provider, setProvider] = useState("forgejo");
  const [credName, setCredName] = useState("forgejo-pat");
  const [baseURL, setBaseURL] = useState("");
  const [token, setToken] = useState("");
  const [remoteOrgs, setRemoteOrgs] = useState<RemoteOrg[]>([]);
  const [remoteOrg, setRemoteOrg] = useState("");
  const [repos, setRepos] = useState<RemoteRepo[]>([]);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [query, setQuery] = useState("");
  const [includeArchived, setIncludeArchived] = useState(false);
  const [busy, setBusy] = useState(false);
  const [job, setJob] = useState<ForgeImportJob | null>(null);
  const [items, setItems] = useState<ForgeImportJobItem[]>([]);
  const [pollNote, setPollNote] = useState("");

  const selectedRepos = useMemo(() => repos.filter((r) => selected[r.full_name]), [repos, selected]);
  const browseProviders = useMemo(
    () => providers.filter((p) => p.family === "gitea" || p.family === "github"),
    [providers],
  );

  async function loadCredentials() {
    if (!org) {
      setCredentials([]);
      return;
    }
    const res = await api.listForgeCredentials(org.id);
    const list = res.credentials ?? [];
    setCredentials(list);
    if (!credentialID && list[0]) setCredentialID(list[0].id);
  }

  useEffect(() => {
    void api
      .listSCMProviders()
      .then((res) => setProviders(res.providers ?? []))
      .catch((err) => {
        setProviders([]);
        setError(err instanceof Error ? err.message : "failed to load forge providers");
      });
    void api
      .listForgeOAuthProviders()
      .then((res) => setOauthProviders(res.providers ?? []))
      .catch((err) => {
        setOauthProviders([]);
        setError(err instanceof Error ? err.message : "failed to load forge providers");
      });
    void api
      .githubAppStatus()
      .then(setGithubApp)
      .catch(() => setGithubApp(null));
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const connected = params.get("credential_id");
    const failed = params.get("error");
    const pending = params.get("github_installation");
    if (pending) setPendingInstall(pending);
    if (!connected && !failed && !pending) return;
    window.history.replaceState({}, "", window.location.pathname);
    if (failed) {
      setError(failed);
      return;
    }
    if (connected) setCredentialID(connected);
  }, []);

  useEffect(() => {
    void loadCredentials().catch((err) => setError(err instanceof Error ? err.message : "failed to load credentials"));
  }, [org?.id]);

  useEffect(() => {
    const info = providers.find((p) => p.id === provider);
    if (info?.default_url) setBaseURL(info.default_url);
  }, [provider, providers]);

  useEffect(() => {
    if (!job || !org) return;
    if (job.status === "completed" || job.status === "failed") return;
    let failures = 0;
    const startedAt = Date.now();
    const timer = window.setInterval(() => {
      if (Date.now() - startedAt >= POLL_TIMEOUT_MS) {
        window.clearInterval(timer);
        setPollNote("Import is still running — refresh to check again.");
        return;
      }
      void api
        .getForgeImport(org.id, job.id)
        .then((res) => {
          failures = 0;
          setJob(res.job);
          setItems(res.items ?? []);
          if (res.job.status === "completed" || res.job.status === "failed") {
            void refreshProjects();
          }
        })
        .catch((err) => {
          failures += 1;
          if (failures < POLL_FAILURE_LIMIT) return;
          window.clearInterval(timer);
          setError(err instanceof Error ? err.message : "failed to follow the import job");
        });
    }, 1200);
    return () => window.clearInterval(timer);
  }, [job?.id, job?.status, org?.id]);

  async function createCredential(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      const res = await api.createForgeCredential(org.id, {
        provider,
        kind: "pat",
        name: credName,
        base_url: baseURL,
        access_token: token,
      });
      setToken("");
      setCredentialID(res.credential.id);
      await loadCredentials();
      try {
        const orgsRes = await api.listRemoteOrgs(org.id, res.credential.id);
        setRemoteOrgs(orgsRes.orgs ?? []);
      } catch {
        setRemoteOrgs([]);
      }
      setStep("org");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save credential");
    } finally {
      setBusy(false);
    }
  }

  async function loadRemoteOrgs() {
    if (!org || !credentialID) return;
    setBusy(true);
    try {
      const res = await api.listRemoteOrgs(org.id, credentialID);
      setRemoteOrgs(res.orgs ?? []);
      setStep("org");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to list remote orgs");
    } finally {
      setBusy(false);
    }
  }

  async function loadRepos(nextOrg?: string) {
    if (!org || !credentialID) return;
    const owner = nextOrg ?? remoteOrg;
    if (!owner) return;
    setBusy(true);
    try {
      const res = await api.listRemoteRepos(org.id, credentialID, {
        org: owner,
        q: query,
        limit: 100,
        include_archived: includeArchived,
      });
      const list = res.repos ?? [];
      setRepos(list);
      setSelected((prev) => {
        const next: Record<string, boolean> = {};
        for (const repo of list) {
          if (prev[repo.full_name]) next[repo.full_name] = true;
        }
        return next;
      });
      setStep("repos");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to list remote repos");
    } finally {
      setBusy(false);
    }
  }

  function toggleAll(on: boolean) {
    const next: Record<string, boolean> = {};
    if (on) {
      for (const repo of repos) next[repo.full_name] = true;
    }
    setSelected(next);
  }

  async function startImport() {
    if (!org || !credentialID || !remoteOrg || selectedRepos.length === 0) return;
    setBusy(true);
    try {
      const res = await api.startForgeImport(org.id, {
        credential_id: credentialID,
        remote_org: remoteOrg,
        repos: selectedRepos,
      });
      setJob(res.job);
      setItems([]);
      setPollNote("");
      setStep("progress");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to start import");
    } finally {
      setBusy(false);
    }
  }

  const progressPct =
    job && job.total_count > 0 ? Math.round((job.completed_count / job.total_count) * 100) : 0;

  return (
    <div className={table.stack}>
      <PageHeader
        title="Import from forge"
        description="Connect your forge account or paste a PAT, pick a remote organization, and import each repository as a Shipyard project with webhook wiring."
      />

      {!org ? (
        <EmptyState
          title="No organization yet"
          description={`Signed in as ${user?.username ?? "unknown"}. Importing needs an organization to put the projects in.`}
          action={
            <Link to="/projects">
              <Button type="button" variant="primary">
                Create an organization
              </Button>
            </Link>
          }
        />
      ) : (
        <>
          <ol className={styles.steps} aria-label="Import steps">
            {(
              [
                ["credential", "Credential"],
                ["org", "Remote org"],
                ["repos", "Repositories"],
                ["progress", "Import"],
              ] as const
            ).map(([id, label]) => {
              const active = step === id;
              const done =
                (id === "credential" && (step === "org" || step === "repos" || step === "progress")) ||
                (id === "org" && (step === "repos" || step === "progress")) ||
                (id === "repos" && step === "progress");
              return (
                <li key={id} className={active ? styles.stepActive : done ? styles.stepDone : styles.step}>
                  {label}
                </li>
              );
            })}
          </ol>

          {step === "credential" ? (
            <Panel title="Forge credential" meta={<StatusBadge status="info">{credentials.length}</StatusBadge>}>
              {credentials.length > 0 ? (
                <div className={table.toolbar}>
                  <select
                    className={table.select}
                    value={credentialID}
                    onChange={(e) => setCredentialID(e.target.value)}
                    aria-label="Saved credential"
                  >
                    {credentials.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name} · {c.provider}
                      </option>
                    ))}
                  </select>
                  <Button type="button" variant="primary" loading={busy} onClick={() => void loadRemoteOrgs()}>
                    Use credential
                  </Button>
                </div>
              ) : null}

              {pendingInstall && org ? (
                <div className={styles.connectRow}>
                  <span className={table.muted}>
                    GitHub installation {pendingInstall} is ready to link to {org.slug}
                  </span>
                  <div className={styles.connectButtons}>
                    <Button
                      type="button"
                      variant="primary"
                      loading={busy}
                      onClick={() => {
                        setBusy(true);
                        void api
                          .linkGitHubAppInstall(org.id, pendingInstall)
                          .then(async (res) => {
                            setPendingInstall("");
                            setCredentialID(res.credential.id);
                            await loadCredentials();
                          })
                          .catch((err) => setError(err instanceof Error ? err.message : "failed to link installation"))
                          .finally(() => setBusy(false));
                      }}
                    >
                      Link installation
                    </Button>
                  </div>
                </div>
              ) : null}

              {githubApp?.configured && githubApp.slug && org ? (
                <div className={styles.connectRow}>
                  <span className={table.muted}>Install the GitHub App for org-wide access</span>
                  <div className={styles.connectButtons}>
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => {
                        window.location.href = api.githubAppInstallURL(org.id);
                      }}
                    >
                      Install GitHub App
                    </Button>
                  </div>
                </div>
              ) : null}

              {oauthProviders.length > 0 && org ? (
                <div className={styles.connectRow}>
                  <span className={table.muted}>Connect your forge account</span>
                  <div className={styles.connectButtons}>
                    {oauthProviders.map((p) => (
                      <Button
                        key={p.name}
                        type="button"
                        variant="secondary"
                        onClick={() => {
                          window.location.href = api.forgeOAuthStartURL(org.id, p.name);
                        }}
                      >
                        Connect {p.name}
                      </Button>
                    ))}
                  </div>
                </div>
              ) : null}

              <form className={table.toolbar} onSubmit={createCredential} style={{ flexDirection: "column", alignItems: "stretch" }}>
                <div className={table.formRow}>
                  <select className={table.select} value={provider} onChange={(e) => setProvider(e.target.value)} aria-label="Provider">
                    {(browseProviders.length ? browseProviders : providers).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.label}
                      </option>
                    ))}
                  </select>
                  <input
                    className={table.input}
                    placeholder="credential name"
                    value={credName}
                    onChange={(e) => setCredName(e.target.value)}
                    required
                    pattern="[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?"
                  />
                  <input
                    className={table.input}
                    placeholder="base URL"
                    value={baseURL}
                    onChange={(e) => setBaseURL(e.target.value)}
                    required
                  />
                  <input
                    className={table.input}
                    type="password"
                    placeholder="personal access token"
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    required
                    autoComplete="off"
                  />
                  <Button type="submit" variant="primary" loading={busy}>
                    Save & continue
                  </Button>
                </div>
              </form>
            </Panel>
          ) : null}

          {step === "org" ? (
            <Panel title="Remote organization" meta={<StatusBadge status="info">{remoteOrgs.length}</StatusBadge>}>
              {remoteOrgs.length === 0 ? (
                <EmptyState title="No remote orgs returned" description="Enter an org/user login below if the token cannot list memberships." />
              ) : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Login</th>
                      <th>Name</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {remoteOrgs.map((o) => (
                      <tr key={o.login}>
                        <td className="mono">{o.login}</td>
                        <td>{o.name || "—"}</td>
                        <td>
                          <Button
                            type="button"
                            variant="secondary"
                            loading={busy}
                            onClick={() => {
                              setRemoteOrg(o.login);
                              void loadRepos(o.login);
                            }}
                          >
                            Select
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
              <form
                className={table.toolbar}
                onSubmit={(e) => {
                  e.preventDefault();
                  void loadRepos();
                }}
              >
                <input
                  className={table.input}
                  placeholder="org or user login"
                  value={remoteOrg}
                  onChange={(e) => setRemoteOrg(e.target.value)}
                  required
                />
                <Button type="submit" variant="primary" loading={busy}>
                  Load repositories
                </Button>
                <Button type="button" variant="ghost" onClick={() => setStep("credential")}>
                  Back
                </Button>
              </form>
            </Panel>
          ) : null}

          {step === "repos" ? (
            <Panel
              title={`Repositories · ${remoteOrg}`}
              meta={<StatusBadge status="info">{selectedRepos.length}/{repos.length}</StatusBadge>}
              actions={
                <div className={table.formRow}>
                  <input
                    className={table.input}
                    placeholder="filter"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                  <label className={table.muted} style={{ display: "inline-flex", gap: 6, alignItems: "center" }}>
                    <input type="checkbox" checked={includeArchived} onChange={(e) => setIncludeArchived(e.target.checked)} />
                    Archived
                  </label>
                  <Button type="button" variant="secondary" loading={busy} onClick={() => void loadRepos()}>
                    Refresh
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => toggleAll(true)}>
                    Select all
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => toggleAll(false)}>
                    Clear
                  </Button>
                  <Button type="button" variant="primary" loading={busy} disabled={selectedRepos.length === 0} onClick={() => void startImport()}>
                    Import {selectedRepos.length || ""}
                  </Button>
                </div>
              }
            >
              {repos.length === 0 ? (
                <EmptyState title="No repositories" description="Try another org or clear the filter." />
              ) : (
                <div className={styles.repoList}>
                  {repos.map((repo) => (
                    <label key={repo.full_name} className={styles.repoRow}>
                      <input
                        type="checkbox"
                        checked={Boolean(selected[repo.full_name])}
                        onChange={(e) => setSelected((prev) => ({ ...prev, [repo.full_name]: e.target.checked }))}
                      />
                      <span className={styles.repoMeta}>
                        <span className={styles.repoName}>{repo.full_name}</span>
                        <span className={styles.repoDesc}>{repo.description || (repo.private ? "private" : "public")}</span>
                      </span>
                      <span className={table.muted}>{repo.archived ? "archived" : repo.default_branch || ""}</span>
                    </label>
                  ))}
                </div>
              )}
              <div className={table.toolbar}>
                <Button type="button" variant="ghost" onClick={() => setStep("org")}>
                  Back
                </Button>
              </div>
            </Panel>
          ) : null}

          {step === "progress" && job ? (
            <Panel title="Import progress" meta={<StatusBadge status={runStatus(job.status)}>{job.status}</StatusBadge>}>
              <div className={styles.progress}>
                <div className={styles.barTrack} aria-hidden>
                  <div className={styles.barFill} style={{ width: `${progressPct}%` }} />
                </div>
                <div className={styles.counts}>
                  <span>
                    {job.completed_count}/{job.total_count} processed
                  </span>
                  <span>{job.created_count} created</span>
                  <span>{job.skipped_count} skipped</span>
                  <span>{job.failed_count} failed</span>
                  <span className={table.muted}>{formatTime(job.updated_at)}</span>
                </div>
                {job.error_message ? <div className={table.error}>{job.error_message}</div> : null}
                {pollNote ? <div className={table.muted}>{pollNote}</div> : null}
              </div>
              {items.length > 0 ? (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Repo</th>
                      <th>Status</th>
                      <th>Detail</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((it) => (
                      <tr key={it.id}>
                        <td className="mono">
                          {it.repo_owner}/{it.repo_name}
                        </td>
                        <td>
                          <StatusBadge status={runStatus(it.status)}>{it.status}</StatusBadge>
                        </td>
                        <td className={table.muted}>{it.error_message || "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              ) : null}
              <div className={table.toolbar}>
                <Button type="button" variant="primary" onClick={() => navigate("/projects")}>
                  Back to projects
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    setJob(null);
                    setItems([]);
                    setStep("repos");
                  }}
                >
                  Import more
                </Button>
              </div>
            </Panel>
          ) : null}
        </>
      )}
    </div>
  );
}
