import { FormEvent, useEffect, useState } from "react";
import { IconChevronLeft } from "@tabler/icons-react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { CodeBlock } from "../components/CodeBlock";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { FormField, formStyles as form } from "../components/FormField";
import { PackageBrowser } from "../components/PackageBrowser";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/TableSkeleton";
import { Tabs } from "../components/Tabs";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, ApiToken, OrgArtifact, OrgOCIRepo, OrgPackageRepo, PackageVersion } from "../api";
import { formatBytes, formatTime } from "../lib/format";
import { usePageTitle } from "../lib/usePageTitle";

type Section = "packages" | "oci" | "artifacts" | "publish";

const SECTIONS = [
  { id: "packages", label: "Packages" },
  { id: "oci", label: "Containers" },
  { id: "artifacts", label: "Build artifacts" },
  { id: "publish", label: "Publish & consume" },
];

function formatLabel(format: string): string {
  switch (format) {
    case "maven":
      return "Maven / Gradle";
    case "npm":
      return "npm";
    case "generic":
      return "Generic";
    default:
      return format;
  }
}

export function RegistryPage() {
  usePageTitle("Registry");
  const { org, projects, user, setError } = useWorkspace();
  const [section, setSection] = useState<Section>("packages");
  const [packages, setPackages] = useState<OrgPackageRepo[]>([]);
  const [oci, setOci] = useState<OrgOCIRepo[]>([]);
  const [buildArtifacts, setBuildArtifacts] = useState<OrgArtifact[]>([]);
  const [selectedPkg, setSelectedPkg] = useState<OrgPackageRepo | null>(null);
  const [versions, setVersions] = useState<PackageVersion[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [selectedOci, setSelectedOci] = useState<OrgOCIRepo | null>(null);
  const [tags, setTags] = useState<string[]>([]);
  const [pkgName, setPkgName] = useState("");
  const [pkgFormat, setPkgFormat] = useState("maven");
  const [pkgProject, setPkgProject] = useState("");
  const [busy, setBusy] = useState(false);
  const [registryToken, setRegistryToken] = useState(() => sessionStorage.getItem("shipyard.registryToken") ?? "");
  const [tokens, setTokens] = useState<ApiToken[]>([]);
  const [revoking, setRevoking] = useState("");
  const [tokenBusy, setTokenBusy] = useState(false);
  const [copied, setCopied] = useState("");
  const [loading, setLoading] = useState(true);

  async function refresh() {
    if (!org) {
      setPackages([]);
      setOci([]);
      setBuildArtifacts([]);
      setLoading(false);
      return;
    }
    try {
      const [p, o, a] = await Promise.allSettled([
        api.listOrgPackages(org.id),
        api.listOrgOCI(org.id),
        api.listOrgArtifacts(org.id),
      ]);
      setPackages(p.status === "fulfilled" ? (p.value.repositories ?? []) : []);
      setOci(o.status === "fulfilled" ? (o.value.repositories ?? []) : []);
      setBuildArtifacts(a.status === "fulfilled" ? (a.value.artifacts ?? []) : []);
      const failed = [
        p.status === "rejected" ? "package repositories" : "",
        o.status === "rejected" ? "containers" : "",
        a.status === "rejected" ? "build artifacts" : "",
      ].filter(Boolean);
      if (failed.length > 0) setError(`failed to load ${failed.join(", ")}`);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load registry"));
  }, [org?.id]);

  useEffect(() => {
    void refreshTokens();
  }, []);

  useEffect(() => {
    if (projects.length === 0) {
      setPkgProject("");
      return;
    }
    if (!projects.some((p) => p.id === pkgProject)) setPkgProject(projects[0].id);
  }, [projects, pkgProject]);

  function changeSection(id: string) {
    setSection(id as Section);
    setSelectedPkg(null);
  }

  async function openPackage(repo: OrgPackageRepo) {
    if (!org) return;
    setSelectedPkg(repo);
    setVersions([]);
    setVersionsLoading(true);
    try {
      const res = await api.listPackageVersions(org.id, repo.project_id, repo.id);
      setVersions(res.versions ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load package versions");
    } finally {
      setVersionsLoading(false);
    }
  }

  async function openOci(repo: OrgOCIRepo) {
    if (!org) return;
    setSelectedOci(repo);
    try {
      const res = await api.listOCITags(org.id, repo.project_id, repo.name);
      setTags(res.tags ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load OCI tags");
    }
  }

  async function createPackage(event: FormEvent) {
    event.preventDefault();
    if (!org || !pkgProject) return;
    setBusy(true);
    try {
      await api.createPackageRepo(org.id, pkgProject, { name: pkgName, format: pkgFormat });
      setPkgName("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create package repository");
    } finally {
      setBusy(false);
    }
  }

  async function generateRegistryToken() {
    if (!org) return;
    setTokenBusy(true);
    setCopied("");
    try {
      const res = await api.createToken({ name: `${org.slug}-registry`, scopes: ["registry:read", "registry:write"] });
      setRegistryToken(res.token);
      sessionStorage.setItem("shipyard.registryToken", res.token);
      await refreshTokens();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to generate registry token");
    } finally {
      setTokenBusy(false);
    }
  }

  async function refreshTokens() {
    try {
      const res = await api.listTokens();
      setTokens(res.tokens ?? []);
    } catch {
      setTokens([]);
    }
  }

  async function revoke(id: string) {
    setRevoking(id);
    try {
      await api.revokeToken(id);
      await refreshTokens();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to revoke token");
    } finally {
      setRevoking("");
    }
  }

  function forgetToken() {
    sessionStorage.removeItem("shipyard.registryToken");
    setRegistryToken("");
  }

  async function copy(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      window.setTimeout(() => setCopied(""), 2000);
    } catch {
      setError("clipboard unavailable — select the text and copy manually");
    }
  }

  const mavenRow = packages.find((p) => p.format === "maven");
  const mavenRepo = mavenRow?.name ?? "<repo>";
  const mavenProject = mavenRow?.project_slug ?? "<project>";
  const apiBase = window.location.origin;
  const gradleBlock = `repositories {
    maven {
        url = uri("${apiBase}/repository/maven/${org?.slug}/${mavenProject}/${mavenRepo}")
        credentials {
            username = providers.gradleProperty("shipyardUser").get()
            password = providers.gradleProperty("shipyardToken").get()
        }
    }
}`;
  const gradleProps = `shipyardUser=${user.username}
shipyardToken=${registryToken}`;
  const m2Settings = `<settings>
  <servers>
    <server>
      <id>shipyard-${mavenRepo}</id>
      <username>${user.username}</username>
      <password>${registryToken}</password>
    </server>
  </servers>
</settings>`;

  return (
    <div className={table.stack}>
      <PageHeader
        title="Registry"
        description={
          org ? (
            <>
              Packages, containers, and build artifacts for <strong>{org.name}</strong>. Maven layout also serves Gradle.
            </>
          ) : (
            "Packages, containers, and build artifacts for your organization. Maven layout also serves Gradle."
          )
        }
      />

      {!org ? (
        <EmptyState title="No organization selected" />
      ) : (
        <>
          <Tabs tabs={SECTIONS} active={section} onChange={changeSection} />

          {section !== "packages" ? null : selectedPkg ? (
            <div className={table.stackTight}>
              <div className={table.backRow}>
                <Button
                  variant="secondary"
                  icon={<IconChevronLeft size={16} stroke={1.75} />}
                  onClick={() => setSelectedPkg(null)}
                >
                  All repositories
                </Button>
                <strong>{selectedPkg.name}</strong>
                <span className={table.muted}>
                  {selectedPkg.project_name} · {formatLabel(selectedPkg.format)}
                </span>
              </div>
              <Panel>
                {versionsLoading ? (
                  <TableSkeleton />
                ) : (
                  <PackageBrowser
                    key={selectedPkg.id}
                    orgSlug={org.slug}
                    projectSlug={selectedPkg.project_slug}
                    repoName={selectedPkg.name}
                    format={selectedPkg.format}
                    versions={versions}
                  />
                )}
              </Panel>
            </div>
          ) : (
            <Panel title="Package repositories" meta={<StatusBadge status="info">{packages.length}</StatusBadge>}>
              <form className={form.row} onSubmit={createPackage}>
                <FormField label="Project" htmlFor="pkg-project">
                  <select
                    id="pkg-project"
                    className={table.select}
                    value={pkgProject}
                    onChange={(e) => setPkgProject(e.target.value)}
                    required
                  >
                    {projects.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                </FormField>
                <FormField label="Repository name" htmlFor="pkg-name">
                  <input
                    id="pkg-name"
                    className={table.input}
                    value={pkgName}
                    onChange={(e) => setPkgName(e.target.value)}
                    required
                  />
                </FormField>
                <FormField label="Format" htmlFor="pkg-format">
                  <select
                    id="pkg-format"
                    className={table.select}
                    value={pkgFormat}
                    onChange={(e) => setPkgFormat(e.target.value)}
                  >
                    <option value="maven">Maven / Gradle</option>
                    <option value="npm">npm</option>
                    <option value="generic">Generic</option>
                  </select>
                </FormField>
                <div className={form.action}>
                  <Button type="submit" variant="primary" loading={busy}>
                    Create repository
                  </Button>
                </div>
              </form>
              {loading ? (
                <TableSkeleton />
              ) : packages.length === 0 ? (
                <EmptyState
                  title="No package repositories"
                  description="Create one above to publish Maven, npm, or generic artifacts from any project in this organization."
                />
              ) : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>Repository</th>
                      <th>Format</th>
                      <th className={table.actionCol} />
                    </tr>
                  </thead>
                  <tbody>
                    {packages.map((p) => (
                      <tr key={p.id}>
                        <td className={table.muted}>{p.project_name}</td>
                        <td>{p.name}</td>
                        <td className={table.muted}>{formatLabel(p.format)}</td>
                        <td className={table.actionCol}>
                          <Button variant="secondary" size="compact" onClick={() => void openPackage(p)}>
                            Browse
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
            </Panel>
          )}

          {section !== "oci" ? null : (
            <Panel title="OCI repositories" meta={<StatusBadge status="info">{oci.length}</StatusBadge>}>
              {loading ? (
                <TableSkeleton />
              ) : oci.length === 0 ? (
                <EmptyState
                  title="No OCI repositories"
                  description={`Push with docker push <host>/${org.slug}/<project>/<name>:<tag> after docker login <host>.`}
                />
              ) : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>Repository</th>
                      <th className={table.actionCol} />
                    </tr>
                  </thead>
                  <tbody>
                    {oci.map((r) => (
                      <tr key={r.id}>
                        <td className={table.muted}>{r.project_name}</td>
                        <td className="mono">{r.name}</td>
                        <td className={table.actionCol}>
                          <Button variant="secondary" size="compact" onClick={() => void openOci(r)}>
                            Tags
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
              {selectedOci ? (
                <div className={table.toolbar}>
                  <strong className="mono">{selectedOci.name}</strong>
                  {tags.length === 0 ? (
                    <span className={table.muted}>No tags</span>
                  ) : (
                    tags.map((t) => (
                      <span key={t} className="mono">
                        {t}
                      </span>
                    ))
                  )}
                </div>
              ) : null}
            </Panel>
          )}

          {section !== "artifacts" ? null : (
            <Panel title="Build artifacts" meta={<StatusBadge status="info">{buildArtifacts.length}</StatusBadge>}>
              {loading ? (
                <TableSkeleton />
              ) : buildArtifacts.length === 0 ? (
                <EmptyState
                  title="No artifacts yet"
                  description="Pipeline jobs upload them with the artifacts step."
                />
              ) : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>Name</th>
                      <th>Digest</th>
                      <th>Size</th>
                      <th className={table.actionCol} />
                    </tr>
                  </thead>
                  <tbody>
                    {buildArtifacts.map((a) => (
                      <tr key={a.id}>
                        <td className={table.muted}>{a.project_name}</td>
                        <td>{a.name}</td>
                        <td className="mono" title={a.digest}>
                          {a.digest.slice(0, 16)}
                        </td>
                        <td>{formatBytes(a.size_bytes)}</td>
                        <td className={table.actionCol}>
                          <a href={`/api/v1/orgs/${org.id}/projects/${a.project_id}/artifacts/${a.id}/download`}>
                            <Button variant="secondary" size="compact">
                              Download
                            </Button>
                          </a>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
            </Panel>
          )}

          {section !== "publish" ? null : (
            <Panel
              title="Publish & consume"
              actions={
                <Button variant="primary" loading={tokenBusy} onClick={() => void generateRegistryToken()}>
                  Generate registry token
                </Button>
              }
            >
              <p className={table.muted}>
                Generate a registry token, then drop it in your global <code className="mono">~/.gradle/gradle.properties</code> and{" "}
                <code className="mono">~/.m2/settings.xml</code> — no passwords in project files. The secret is shown once; revoke and
                generate a new one if you lose it.
              </p>

              {tokens.length === 0 ? null : (
                <DataTable>
                  <thead>
                    <tr>
                      <th>Token</th>
                      <th>Scopes</th>
                      <th>Created</th>
                      <th>Last used</th>
                      <th className={table.actionCol} />
                    </tr>
                  </thead>
                  <tbody>
                    {tokens.map((t) => (
                      <tr key={t.id}>
                        <td>
                          <span className={table.formRow}>
                            <strong>{t.name}</strong>
                            <code className="mono">{t.prefix}…</code>
                          </span>
                        </td>
                        <td className={table.muted}>{(t.scopes ?? []).join(", ") || "full access"}</td>
                        <td className={table.muted}>{formatTime(t.created_at)}</td>
                        <td className={table.muted}>{t.last_used_at ? formatTime(t.last_used_at) : "never"}</td>
                        <td className={table.actionCol}>
                          <Button
                            type="button"
                            variant="secondary"
                            size="compact"
                            loading={revoking === t.id}
                            onClick={() => void revoke(t.id)}
                          >
                            Revoke
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </DataTable>
              )}
              {registryToken ? (
                <div className={table.stack}>
                  <div className={table.toolbar}>
                    <strong>Registry token</strong>
                    <Button variant="secondary" size="compact" onClick={() => void copy(registryToken, "token")}>
                      {copied === "token" ? "Copied" : "Copy token"}
                    </Button>
                    <Button variant="ghost" size="compact" onClick={forgetToken}>
                      Hide
                    </Button>
                  </div>
                  <div className={table.toolbar}>
                    <strong>build.gradle.kts</strong>
                    <Button variant="secondary" onClick={() => void copy(gradleBlock, "gradle")}>
                      {copied === "gradle" ? "Copied" : "Copy"}
                    </Button>
                  </div>
                  <CodeBlock language="kotlin" code={gradleBlock} />
                  <div className={table.toolbar}>
                    <strong>~/.gradle/gradle.properties</strong>
                    <Button variant="secondary" onClick={() => void copy(gradleProps, "props")}>
                      {copied === "props" ? "Copied" : "Copy"}
                    </Button>
                  </div>
                  <CodeBlock language="properties" code={gradleProps} />
                  <div className={table.toolbar}>
                    <strong>~/.m2/settings.xml</strong>
                    <Button variant="secondary" onClick={() => void copy(m2Settings, "m2")}>
                      {copied === "m2" ? "Copied" : "Copy"}
                    </Button>
                  </div>
                  <CodeBlock language="markup" code={m2Settings} />
                </div>
              ) : (
                <EmptyState
                  title={tokens.length ? "Secret not shown" : "No token yet"}
                  description={
                    tokens.length
                      ? "Your tokens are listed above, but the secret is only shown when it is created. Generate a new one to see the Gradle and Maven config again."
                      : "Generate a registry token to see the global Gradle and Maven config."
                  }
                />
              )}
            </Panel>
          )}
        </>
      )}
    </div>
  );
}
