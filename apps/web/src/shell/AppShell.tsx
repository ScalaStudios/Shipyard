import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Link, NavLink, useLocation } from "react-router-dom";
import { IconLogout, IconMenu2 } from "@tabler/icons-react";
import { NotificationBell } from "../app/components/NotificationBell";
import { useWorkspace } from "../app/context/WorkspaceContext";
import { NAV_GROUPS, SETTINGS_ITEM, type NavItem } from "./nav";
import { buildPathSegments } from "./path";
import styles from "./AppShell.module.css";

const MENU_BREAKPOINT = 960;

function NavRow({ item, onNavigate }: { item: NavItem; onNavigate: () => void }) {
  const Icon = item.icon;
  return (
    <NavLink
      to={item.to}
      end={item.to === "/"}
      className={({ isActive }) => (isActive ? styles.navActive : styles.navItem)}
      onClick={onNavigate}
    >
      <Icon size={16} stroke={1.5} />
      <span className={styles.navLabel}>{item.label}</span>
    </NavLink>
  );
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
  const [menuOpen, setMenuOpen] = useState(false);
  const { user, orgs, projects, org, project, setOrgID, setProjectID, error, clearError } = useWorkspace();
  const segments = buildPathSegments({
    orgSlug: org?.slug,
    projectSlug: project?.slug,
    pathname: location.pathname,
  });

  function closeMenu() {
    setMenuOpen(false);
  }

  useEffect(() => {
    closeMenu();
  }, [location.pathname]);

  useEffect(() => {
    function onResize() {
      if (window.innerWidth > MENU_BREAKPOINT) closeMenu();
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") closeMenu();
    }
    window.addEventListener("resize", onResize);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("resize", onResize);
      window.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  const displayName = user.display_name || user.username;
  const initial = displayName.trim().slice(0, 1).toUpperCase() || "?";

  const scrimPortal =
    menuOpen && typeof document !== "undefined"
      ? createPortal(
          <button type="button" className={styles.scrim} aria-label="Close navigation menu" onClick={closeMenu} />,
          document.body,
        )
      : null;

  return (
    <div className={styles.shell}>
      <aside className={menuOpen ? styles.sidebarOpen : styles.sidebar}>
        <div className={styles.brand}>
          <img className={styles.markImg} src="/shipyard-mark.svg" width={28} height={28} alt="" />
          <strong className={styles.brandName}>Shipyard</strong>
        </div>

        <nav className={styles.nav} aria-label="Primary">
          {NAV_GROUPS.map((group, index) => (
            <div key={group.label ?? `group-${index}`} className={styles.navGroup}>
              {group.label ? <span className={styles.navGroupLabel}>{group.label}</span> : null}
              {group.items.map((item) => (
                <NavRow key={item.id} item={item} onNavigate={closeMenu} />
              ))}
            </div>
          ))}
        </nav>

        <div className={styles.sidebarFoot}>
          <div className={styles.navGroup}>
            <NavRow item={SETTINGS_ITEM} onNavigate={closeMenu} />
          </div>
          <div className={styles.account}>
            <span className={styles.avatar} aria-hidden="true">
              {initial}
            </span>
            <span className={styles.accountName}>{displayName}</span>
            <button type="button" className={styles.signOut} onClick={onLogout} aria-label="Sign out">
              <IconLogout size={16} stroke={1.75} aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      {scrimPortal}

      <div className={styles.main}>
        <header className={styles.topbar}>
          <button
            type="button"
            className={styles.menuButton}
            aria-label="Open navigation menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((open) => !open)}
          >
            <IconMenu2 size={16} stroke={1.5} />
          </button>

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
            {orgs.length === 0 ? (
              <Link className={styles.contextCta} to="/projects">
                Create organization
              </Link>
            ) : (
              <label className={styles.contextLabel}>
                <span className={styles.srOnly}>Organization</span>
                <select className={styles.select} value={org?.id ?? ""} onChange={(e) => setOrgID(e.target.value)}>
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {orgs.length > 0 && projects.length === 0 ? (
              <Link className={styles.contextCta} to="/projects/import">
                Import repositories
              </Link>
            ) : null}
            {projects.length > 0 ? (
              <label className={styles.contextLabel}>
                <span className={styles.srOnly}>Project</span>
                <select
                  className={styles.select}
                  value={project?.id ?? ""}
                  onChange={(e) => setProjectID(e.target.value)}
                >
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
    </div>
  );
}
