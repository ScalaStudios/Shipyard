import type { ReactNode } from "react";
import { Link, NavLink, useLocation } from "react-router-dom";
import { IconMenu2 } from "@tabler/icons-react";
import { NotificationBell } from "../app/components/NotificationBell";
import { useWorkspace } from "../app/context/WorkspaceContext";
import { NAV_ITEMS } from "./nav";
import { buildPathSegments } from "./path";
import styles from "./AppShell.module.css";

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
  const { user, orgs, projects, org, project, setOrgID, setProjectID, error, clearError } = useWorkspace();
  const segments = buildPathSegments({
    orgSlug: org?.slug,
    projectSlug: project?.slug,
    pathname: location.pathname,
  });

  return (
    <div className={styles.shell}>
      <div className={styles.chrome}>
        <header className={styles.topbar}>
          <div className={styles.brand}>
            <img className={styles.markImg} src="/shipyard-mark.svg" width={28} height={28} alt="" />
            <strong className={styles.brandName}>Shipyard</strong>
          </div>

          <div className={styles.navCluster}>
            <nav className={styles.navList} aria-label="Primary">
              {NAV_ITEMS.map((item) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    key={item.id}
                    to={item.to}
                    end={item.to === "/"}
                    className={({ isActive }) => (isActive ? styles.navActive : styles.navItem)}
                    title={item.label}
                  >
                    <Icon size={16} stroke={1.5} />
                    <span className={styles.navLabel}>{item.label}</span>
                  </NavLink>
                );
              })}
            </nav>

            <details key={location.pathname} className={styles.menu}>
              <summary className={styles.menuSummary} aria-label="Open navigation menu">
                <IconMenu2 size={16} stroke={1.5} />
                <span className={styles.menuLabel}>Menu</span>
              </summary>
              <div className={styles.menuPanel}>
                {NAV_ITEMS.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.id}
                      to={item.to}
                      end={item.to === "/"}
                      className={({ isActive }) => (isActive ? styles.menuActive : styles.menuItem)}
                    >
                      <Icon size={16} stroke={1.5} />
                      <span>{item.label}</span>
                    </NavLink>
                  );
                })}
              </div>
            </details>
          </div>

          <div className={styles.actions}>
            <NotificationBell />
            <div className={styles.themeToggle}>{themeToggle}</div>
            <div className={styles.account}>
              <span className={styles.accountName}>{user.display_name || user.username}</span>
              <button type="button" className={styles.signOut} onClick={onLogout}>
                <span className={styles.signOutLabel}>Sign out</span>
              </button>
            </div>
          </div>
        </header>

        <div className={styles.pathbar}>
          <nav className={styles.path} aria-label="Location">
            <span className={styles.pathRoot}>/</span>
            {segments.map((segment, index) => (
              <span key={`${segment.label}-${index}`} className={styles.pathChunk}>
                {segment.to && index < segments.length - 1 ? (
                  <Link to={segment.to} className={styles.pathLink}>
                    {segment.label}
                  </Link>
                ) : (
                  <span className={styles.pathCurrent}>{segment.label}</span>
                )}
                {index < segments.length - 1 ? <span className={styles.pathSep}>/</span> : null}
              </span>
            ))}
          </nav>

          <div className={styles.context}>
            <label className={styles.contextLabel}>
              <span className={styles.srOnly}>Organization</span>
              <select
                className={styles.select}
                value={org?.id ?? ""}
                onChange={(e) => setOrgID(e.target.value)}
                disabled={orgs.length === 0}
              >
                {orgs.length === 0 ? <option value="">No organizations</option> : null}
                {orgs.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.contextLabel}>
              <span className={styles.srOnly}>Project</span>
              <select
                className={styles.select}
                value={project?.id ?? ""}
                onChange={(e) => setProjectID(e.target.value)}
                disabled={!org || projects.length === 0}
              >
                {!org || projects.length === 0 ? <option value="">No projects</option> : null}
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
        </div>
      </div>

      <main className={styles.content}>
        {error ? (
          <div className={styles.banner} role="alert">
            <span>{error}</span>
            <button type="button" className={styles.signOut} onClick={clearError}>
              Dismiss
            </button>
          </div>
        ) : null}
        <div key={location.pathname} className={styles.route}>
          {children}
        </div>
      </main>
    </div>
  );
}
