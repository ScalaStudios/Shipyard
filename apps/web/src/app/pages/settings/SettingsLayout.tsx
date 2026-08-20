import { NavLink, Outlet } from "react-router-dom";
import { useWorkspace } from "../../context/WorkspaceContext";
import styles from "./SettingsLayout.module.css";

type SettingsLink = { to: string; label: string; end?: boolean; adminOnly?: boolean };

const links: SettingsLink[] = [
  { to: "/settings", end: true, label: "Account" },
  { to: "/settings/instance", label: "Instance", adminOnly: true },
  { to: "/settings/authentication", label: "Authentication", adminOnly: true },
  { to: "/settings/members", label: "Members" },
  { to: "/settings/secrets", label: "Secrets" },
  { to: "/settings/integrations", label: "Integrations" },
  { to: "/settings/webhooks", label: "Webhooks" },
  { to: "/settings/notifications", label: "Notifications" },
];

export function SettingsLayout() {
  const { user } = useWorkspace();
  const visible = links.filter((link) => !link.adminOnly || user.is_admin);

  return (
    <div className={styles.layout}>
      <aside className={styles.side} aria-label="Settings">
        <div className={styles.sideTitle}>Settings</div>
        <nav className={styles.sideNav}>
          {visible.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              end={link.end}
              className={({ isActive }) => (isActive ? styles.sideActive : styles.sideItem)}
            >
              {link.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className={styles.main}>
        <Outlet />
      </div>
    </div>
  );
}
