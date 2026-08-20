import { useMemo, useState } from "react";
import { Button, EmptyState } from "@shipyard/ui";
import { CodeBlock } from "./CodeBlock";
import { DataTable } from "./DataTable";
import table from "./DataTable.module.css";
import styles from "./PackageBrowser.module.css";
import { Tabs } from "./Tabs";
import { useWorkspace } from "../context/WorkspaceContext";
import { formatBytes, formatTime } from "../lib/format";
import type { PackageVersion } from "../api";

const INTERNAL_VERSIONS = new Set(["metadata", "file"]);

type VersionEntry = {
  version: string;
  createdAt: string;
  files: PackageVersion[];
};

type Artifact = {
  name: string;
  group: string;
  artifact: string;
  versions: VersionEntry[];
};

function splitCoordinate(name: string): [string, string] {
  const at = name.indexOf(":");
  return at === -1 ? ["", name] : [name.slice(0, at), name.slice(at + 1)];
}

export function PackageBrowser({
  orgSlug,
  projectSlug,
  repoName,
  format,
  versions,
}: {
  orgSlug: string;
  projectSlug: string;
  repoName: string;
  format: string;
  versions: PackageVersion[];
}) {
  const { setError } = useWorkspace();
  const [query, setQuery] = useState("");
  const [selectedName, setSelectedName] = useState("");
  const [selectedVersion, setSelectedVersion] = useState("");
  const [copied, setCopied] = useState("");
  const [snippetTab, setSnippetTab] = useState(format === "npm" ? "npm" : "kts");

  const artifacts = useMemo<Artifact[]>(() => {
    const byName = new Map<string, Map<string, PackageVersion[]>>();
    for (const file of versions) {
      if (INTERNAL_VERSIONS.has(file.version)) continue;
      const name = file.name && file.name.length > 0 ? file.name : repoName;
      let byVersion = byName.get(name);
      if (!byVersion) {
        byVersion = new Map<string, PackageVersion[]>();
        byName.set(name, byVersion);
      }
      const bucket = byVersion.get(file.version);
      if (bucket) bucket.push(file);
      else byVersion.set(file.version, [file]);
    }
    return Array.from(byName.entries())
      .map(([name, byVersion]) => {
        const [group, artifact] = splitCoordinate(name);
        const entries = Array.from(byVersion.entries())
          .map(([version, files]) => ({
            version,
            createdAt: files.reduce((latest, f) => (f.created_at && f.created_at > latest ? f.created_at : latest), ""),
            files: files.slice().sort((a, b) => (a.filename ?? "").localeCompare(b.filename ?? "")),
          }))
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt) || b.version.localeCompare(a.version));
        return { name, group, artifact, versions: entries };
      })
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [versions, repoName]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return artifacts;
    return artifacts.filter((a) => a.name.toLowerCase().includes(q));
  }, [artifacts, query]);

  const selected = artifacts.find((a) => a.name === selectedName) ?? null;
  const entry = selected?.versions.find((v) => v.version === selectedVersion) ?? null;

  async function copy(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      window.setTimeout(() => setCopied(""), 2000);
    } catch {
      setError("clipboard unavailable — select the text and copy manually");
    }
  }

  function selectArtifact(a: Artifact) {
    setSelectedName(a.name);
    setSelectedVersion(a.versions[0]?.version ?? "");
  }

  const origin = window.location.origin;
  const registryURL = `${origin}/repository/${format === "npm" ? "npm" : "maven"}/${orgSlug}/${projectSlug}/${repoName}`;

  function fileURL(file: PackageVersion): string {
    if (!selected || !entry) return registryURL;
    const filename = file.filename ?? "";
    if (format === "npm") return `${registryURL}/${selected.name}/-/${filename}`;
    const path = selected.group ? `${selected.group.replaceAll(".", "/")}/${selected.artifact}` : selected.artifact;
    return `${registryURL}/${path}/${entry.version}/${filename}`;
  }

  const gradleKts = selected && entry
    ? `// Source: ${registryURL}
implementation("${selected.group}:${selected.artifact}:${entry.version}")`
    : "";
  const gradleGroovy = selected && entry
    ? `// Source: ${registryURL}
implementation '${selected.group}:${selected.artifact}:${entry.version}'`
    : "";
  const mavenXML = selected && entry
    ? `<dependency>
  <groupId>${selected.group}</groupId>
  <artifactId>${selected.artifact}</artifactId>
  <version>${entry.version}</version>
</dependency>`
    : "";
  const npmInstall = selected && entry ? `npm install ${selected.name}@${entry.version}` : "";
  const npmrc = `registry=${registryURL}`;
  const repoBlock = `repositories {
    maven {
        url = uri("${registryURL}")
        credentials {
            username = providers.gradleProperty("shipyardUser").get()
            password = providers.gradleProperty("shipyardToken").get()
        }
    }
}`;

  const snippets =
    format === "npm"
      ? [
          { id: "npm", label: "npm", language: "bash", code: npmInstall },
          { id: "npmrc", label: ".npmrc", language: "properties", code: npmrc },
        ]
      : [
          ...(selected?.group
            ? [
                { id: "kts", label: "Gradle (Kotlin)", language: "kotlin", code: gradleKts },
                { id: "groovy", label: "Gradle (Groovy)", language: "clike", code: gradleGroovy },
                { id: "maven", label: "Maven", language: "markup", code: mavenXML },
              ]
            : []),
          { id: "repo", label: "Repository", language: "kotlin", code: repoBlock },
        ];
  const snippet = snippets.find((s) => s.id === snippetTab) ?? snippets[0];

  if (artifacts.length === 0) {
    return (
      <EmptyState
        title="No artifacts published"
        description={`Nothing has been published to ${repoName} yet. Publish a build, then refresh to browse coordinates and versions.`}
      />
    );
  }

  return (
    <div className={styles.browser}>
      <div className={styles.tree}>
        <input
          className={table.input}
          placeholder="Filter coordinates"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="Filter artifact coordinates"
        />
        {filtered.length === 0 ? (
          <p className={styles.note}>No coordinate matches “{query.trim()}”.</p>
        ) : (
          <ul className={styles.treeList}>
            {filtered.map((a) => (
              <li key={a.name} className={styles.treeItem}>
                <button
                  type="button"
                  className={`${styles.artifactButton} ${a.name === selectedName ? styles.artifactActive : ""}`}
                  onClick={() => selectArtifact(a)}
                  aria-expanded={a.name === selectedName}
                >
                  <span className={styles.coordinate}>
                    {a.group ? <span className={styles.group}>{a.group}</span> : null}
                    <span className={styles.artifact}>{a.artifact}</span>
                  </span>
                  <span className={styles.count}>{a.versions.length}</span>
                </button>
                {a.name === selectedName ? (
                  <ul className={styles.versionList}>
                    {a.versions.map((v) => (
                      <li key={v.version}>
                        <button
                          type="button"
                          className={`${styles.versionButton} ${v.version === selectedVersion ? styles.versionActive : ""}`}
                          onClick={() => setSelectedVersion(v.version)}
                          aria-current={v.version === selectedVersion}
                        >
                          <span>{v.version}</span>
                          <span>{v.files.length}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </div>

      {selected && entry ? (
        <div className={styles.detail}>
          <header className={styles.detailHead}>
            <h3 className={styles.detailTitle}>{selected.artifact}</h3>
            <div className={styles.detailMeta}>
              {selected.group ? <code>{selected.group}</code> : null}
              <span>Version {entry.version}</span>
              <span>Published {formatTime(entry.createdAt)}</span>
            </div>
          </header>

          <section className={styles.section}>
            <span className={styles.sectionLabel}>Files</span>
            <div className={styles.tableFrame}>
              <DataTable>
                <thead>
                  <tr>
                    <th>Filename</th>
                    <th>Size</th>
                    <th>Digest</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {entry.files.map((f) => (
                    <tr key={f.id}>
                      <td className="mono">{f.filename ?? "—"}</td>
                      <td>{typeof f.size_bytes === "number" ? formatBytes(f.size_bytes) : "—"}</td>
                      <td>
                        {f.digest ? (
                          <button
                            type="button"
                            className={styles.digest}
                            onClick={() => void copy(f.digest ?? "", `digest:${f.id}`)}
                            title={f.digest}
                          >
                            {copied === `digest:${f.id}` ? "Copied" : f.digest.slice(0, 16)}
                          </button>
                        ) : (
                          <span className={table.muted}>—</span>
                        )}
                      </td>
                      <td>
                        <button type="button" className={table.rowButton} onClick={() => void copy(fileURL(f), `url:${f.id}`)}>
                          {copied === `url:${f.id}` ? "Copied" : "Copy URL"}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            </div>
          </section>

          <section className={styles.section}>
            <span className={styles.sectionLabel}>Add to your build</span>
            <div className={styles.snippetHead}>
              <Tabs tabs={snippets} active={snippet.id} onChange={setSnippetTab} />
              <Button variant="secondary" onClick={() => void copy(snippet.code, `snippet:${snippet.id}`)}>
                {copied === `snippet:${snippet.id}` ? "Copied" : "Copy"}
              </Button>
            </div>
            <CodeBlock language={snippet.language} code={snippet.code} />
            {format === "npm" || selected.group ? null : (
              <p className={styles.note}>
                This repository stores files without Maven coordinates — use the file URLs above to download directly.
              </p>
            )}
            {snippet.id === "repo" ? (
              <p className={styles.note}>
                Generate the matching <code className="mono">shipyardUser</code> and <code className="mono">shipyardToken</code> values under
                the Publish &amp; consume tab.
              </p>
            ) : null}
          </section>
        </div>
      ) : (
        <EmptyState title="Select an artifact" description="Pick a coordinate on the left to see its files, checksums, and build snippets." />
      )}
    </div>
  );
}
