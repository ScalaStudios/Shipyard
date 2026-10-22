import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { Link, NavLink, useLocation } from "react-router-dom";
import { LogOut, Menu, Search } from "lucide-react";
import { NotificationBell } from "../app/components/NotificationBell";
import { useWorkspace } from "../app/context/WorkspaceContext";
import { CommandPalette } from "./CommandPalette";
import { NAV_GROUPS } from "./nav";
import { buildPathSegments } from "./path";
import styles from "./AppShell.module.css";

function initials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return ((parts[0]?.[0] ?? "") + (parts[1]?.[0] ?? "")).toUpperCase() || "?";
}

export function AppShell({
  children,
  themeToggle,
  onLogout,
}: {
  children: ReactNode;
  themeToggle: ReactNode;
  onLogout: () => void;
}) {
  const location = useLocation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const { user, orgs, projects, org, project, loading, setOrgID, setProjectID, error, clearError } = useWorkspace();
  const segments = buildPathSegments({
    orgSlug: org?.slug,
    projectSlug: project?.slug,
    pathname: location.pathname,
  });
  const displayName = user.display_name || user.username;

  useEffect(() => {
    setDrawerOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen((open) => !open);
      } else if (event.key === "Escape") {
        setDrawerOpen(false);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar} data-open={drawerOpen || undefined} aria-label="Sidebar">
        <div className={styles.sideHead}>
          <img className={styles.mark} src="/shipyard-mark.svg" width={32} height={32} alt="" />
          <div className={styles.sideBrand}>
            <strong className={styles.brandName}>Shipyard</strong>
            {loading ? null : orgs.length === 0 ? (
              <Link className={styles.orgLink} to="/projects">
                Create organization
              </Link>
            ) : (
              <label className={styles.orgLabel}>
                <span className={styles.srOnly}>Organization</span>
                <select className={styles.orgSelect} value={org?.id ?? ""} onChange={(e) => setOrgID(e.target.value)}>
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
        </div>

        <button type="button" className={styles.search} onClick={() => setPaletteOpen(true)}>
          <Search size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>Search</span>
          <kbd className={styles.kbd}>⌘K</kbd>
        </button>

        <nav className={styles.nav} aria-label="Primary">
          {NAV_GROUPS.map((group) => (
            <div key={group.id} className={styles.group}>
              {group.label ? <div className={styles.groupLabel}>{group.label}</div> : null}
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    key={item.id}
                    to={item.to}
                    end={item.to === "/"}
                    className={({ isActive }) => (isActive ? styles.navActive : styles.navItem)}
                  >
                    <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
                    <span>{item.label}</span>
                  </NavLink>
                );
              })}
            </div>
          ))}
        </nav>

        <div className={styles.sideFoot}>
          <a className={styles.footLink} href="/api/docs" target="_blank" rel="noreferrer">
            API reference
          </a>
        </div>
      </aside>

      {drawerOpen ? (
        <button type="button" className={styles.scrim} aria-label="Close navigation" onClick={() => setDrawerOpen(false)} />
      ) : null}

      <div className={styles.main}>
        <header className={styles.topbar}>
          <button
            type="button"
            className={styles.iconButton}
            aria-label="Open navigation"
            aria-expanded={drawerOpen}
            onClick={() => setDrawerOpen(true)}
          >
            <Menu size={16} strokeWidth={1.75} />
          </button>

          <nav className={styles.path} aria-label="Location">
            {segments.map((segment, index) => (
              <span key={`${segment.label}-${index}`} className={styles.pathChunk}>
                {segment.to && index < segments.length - 1 ? (
                  <Link to={segment.to} className={styles.pathLink}>
                    {segment.label}
                  </Link>
                ) : (
                  <span className={styles.pathCurrent} aria-current="page">
                    {segment.label}
                  </span>
                )}
                {index < segments.length - 1 ? <span className={styles.pathSep}>/</span> : null}
              </span>
            ))}
          </nav>

          <div className={styles.actions}>
            {!loading && orgs.length > 0 && projects.length === 0 ? (
              <Link className={styles.contextCta} to="/projects/import">
                Import from forge
              </Link>
            ) : null}
            {projects.length > 0 ? (
              <label className={styles.contextLabel}>
                <span className={styles.srOnly}>Project</span>
                <select className={styles.select} value={project?.id ?? ""} onChange={(e) => setProjectID(e.target.value)}>
                  <option value="">Select project</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <NotificationBell />
            <div className={styles.themeToggle}>{themeToggle}</div>
            <Link className={styles.account} to="/settings" title="Account settings">
              <span className={styles.avatar} aria-hidden="true">
                {initials(displayName)}
              </span>
              <span className={styles.accountName}>{displayName}</span>
            </Link>
            <button type="button" className={styles.iconButton} onClick={onLogout} aria-label="Sign out" title="Sign out">
              <LogOut size={16} strokeWidth={1.75} aria-hidden="true" />
            </button>
          </div>
        </header>

        <main className={styles.content}>
          {error ? (
            <div className={styles.banner} role="alert">
              <span>{error}</span>
              <button type="button" className={styles.bannerDismiss} onClick={clearError}>
                Dismiss
              </button>
            </div>
          ) : null}
          <div key={location.pathname} className={styles.route}>
            {children}
          </div>
        </main>
      </div>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
    </div>
  );
}
