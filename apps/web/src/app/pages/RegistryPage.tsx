import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../components/DataTable";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, OCIRepo, PackageRepo, PackageVersion } from "../api";

function formatLabel(format: string): string {
  switch (format) {
    case "maven":
      return "Maven / Gradle";
    case "npm":
      return "npm";
    case "generic":
      return "generic";
    default:
      return format;
  }
}

export function RegistryPage() {
  const { org, project, setError } = useWorkspace();
  const [packages, setPackages] = useState<PackageRepo[]>([]);
  const [oci, setOci] = useState<OCIRepo[]>([]);
  const [selectedPkg, setSelectedPkg] = useState<PackageRepo | null>(null);
  const [versions, setVersions] = useState<PackageVersion[]>([]);
  const [selectedOci, setSelectedOci] = useState<OCIRepo | null>(null);
  const [tags, setTags] = useState<string[]>([]);
  const [pkgName, setPkgName] = useState("");
  const [pkgFormat, setPkgFormat] = useState("maven");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    if (!org || !project) {
      setPackages([]);
      setOci([]);
      return;
    }
    const [p, o] = await Promise.all([api.listPackages(org.id, project.id), api.listOCI(org.id, project.id)]);
    setPackages(p.repositories ?? []);
    setOci(o.repositories ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load registry"));
  }, [org?.id, project?.id]);

  async function openPackage(repo: PackageRepo) {
    if (!org || !project) return;
    setSelectedPkg(repo);
    try {
      const res = await api.listPackageVersions(org.id, project.id, repo.id);
      setVersions(res.versions ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load package versions");
    }
  }

  async function openOci(repo: OCIRepo) {
    if (!org || !project) return;
    setSelectedOci(repo);
    try {
      const res = await api.listOCITags(org.id, project.id, repo.name);
      setTags(res.tags ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load OCI tags");
    }
  }

  async function createPackage(event: FormEvent) {
    event.preventDefault();
    if (!org || !project) return;
    setBusy(true);
    try {
      await api.createPackageRepo(org.id, project.id, { name: pkgName, format: pkgFormat });
      setPkgName("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create package repository");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title={project ? `${project.name} - Registry` : "Registry"}
        description={
          project ? (
            <>
              Package and OCI repositories for <strong>{project.name}</strong>. Maven layout also serves Gradle.
            </>
          ) : (
            "Package and OCI repositories for the selected project. Maven layout also serves Gradle."
          )
        }
      />

      {!org || !project ? (
        <EmptyState title="Select a project" />
      ) : (
        <>
          <Panel
            title="Package repositories"
            meta={<StatusBadge status="info">{packages.length}</StatusBadge>}
            actions={
              <form className={table.formRow} onSubmit={createPackage}>
                <input className={table.input} placeholder="name" value={pkgName} onChange={(e) => setPkgName(e.target.value)} required />
                <select
                  className={table.select}
                  value={pkgFormat}
                  onChange={(e) => setPkgFormat(e.target.value)}
                  aria-label="Package format"
                >
                  <option value="maven">Maven / Gradle</option>
                  <option value="npm">npm</option>
                  <option value="generic">generic</option>
                </select>
                <Button type="submit" variant="primary" loading={busy}>
                  Create
                </Button>
              </form>
            }
          >
            {packages.length === 0 ? (
              <EmptyState title="No package repositories" />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Format</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {packages.map((p) => (
                    <tr key={p.id}>
                      <td>{p.name}</td>
                      <td className="mono">{formatLabel(p.format)}</td>
                      <td>
                        <button type="button" className={table.rowButton} onClick={() => void openPackage(p)}>
                          Versions
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            )}
            {selectedPkg ? (
              <div className={table.toolbar}>
                <strong>{selectedPkg.name}</strong>
                <span className={table.muted}>{versions.length} versions</span>
                {versions.map((v) => (
                  <span key={v.id} className="mono">
                    {v.version}
                  </span>
                ))}
                {versions.length === 0 ? <span className={table.muted}>No versions published yet.</span> : null}
              </div>
            ) : null}
          </Panel>

          <Panel title="OCI repositories" meta={<StatusBadge status="info">{oci.length}</StatusBadge>}>
            {oci.length === 0 ? (
              <EmptyState title="No OCI repositories" description="Push images via the /v2 distribution API." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {oci.map((r) => (
                    <tr key={r.id}>
                      <td className="mono">{r.name}</td>
                      <td>
                        <button type="button" className={table.rowButton} onClick={() => void openOci(r)}>
                          Tags
                        </button>
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
        </>
      )}
    </div>
  );
}
