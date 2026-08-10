import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { NotificationBell } from "../app/components/NotificationBell";
import { useWorkspace } from "../app/context/WorkspaceContext";
import { NAV_ITEMS } from "./nav";
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
  const active = NAV_ITEMS.find((item) =>
    item.to === "/" ? location.pathname === "/" : location.pathname.startsWith(item.to),
  );
  const title = active?.label ?? "Shipyard";

  return (
    <div className={styles.shell}>
      <aside className={styles.nav} aria-label="Primary">
        <div className={styles.brand}>
          <img className={styles.markImg} src="/shipyard-mark.svg" width={28} height={28} alt="" />
          <div>
            <strong>Shipyard</strong>
            <div className={styles.brandMeta}>Delivery control plane</div>
          </div>
        </div>
        <nav className={styles.navList}>
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.id}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) => (isActive ? styles.navActive : styles.navItem)}
              >
                <Icon size={18} stroke={1.5} />
                <span>{item.label}</span>
              </NavLink>
            );
          })}
        </nav>
      </aside>

      <div className={styles.main}>
        <header className={styles.topbar}>
          <div className={styles.topbarLeft}>
            <div>
              <div className={styles.eyebrow}>
                {org ? org.slug : "no org"}
                {project ? ` / ${project.slug}` : ""}
              </div>
              <div className={styles.title}>{title}</div>
            </div>
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
          <div className={styles.actions}>
            <NotificationBell />
            {themeToggle}
            <div className={styles.account}>
              <span className={styles.accountName}>{user.display_name || user.username}</span>
              <button type="button" className={styles.signOut} onClick={onLogout}>
                Sign out
              </button>
            </div>
          </div>
        </header>

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
    </div>
  );
}
