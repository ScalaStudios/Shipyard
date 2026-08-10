import { NavLink, Outlet } from "react-router-dom";
import styles from "./SettingsLayout.module.css";

const links = [
  { to: "/settings", end: true, label: "Account" },
  { to: "/settings/authentication", label: "Authentication" },
  { to: "/settings/members", label: "Members" },
  { to: "/settings/secrets", label: "Secrets" },
  { to: "/settings/integrations", label: "Integrations" },
  { to: "/settings/webhooks", label: "Webhooks" },
  { to: "/settings/notifications", label: "Notifications" },
];

export function SettingsLayout() {
  return (
    <div className={styles.layout}>
      <aside className={styles.side} aria-label="Settings">
        <div className={styles.sideTitle}>Settings</div>
        <nav className={styles.sideNav}>
          {links.map((link) => (
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
