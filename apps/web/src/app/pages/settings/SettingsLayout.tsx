import { NavLink, Outlet } from "react-router-dom";
import { useWorkspace } from "../../context/WorkspaceContext";
import { usePageTitle } from "../../lib/usePageTitle";
import styles from "./SettingsLayout.module.css";

type SettingsLink = { to: string; label: string; end?: boolean };

function SettingsShell({ title, links }: { title: string; links: SettingsLink[] }) {
  usePageTitle(title);

  return (
    <div>
      <nav className={styles.tabs} aria-label={title}>
        {links.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            end={link.end}
            className={({ isActive }) => (isActive ? styles.tabActive : styles.tab)}
          >
            {link.label}
          </NavLink>
        ))}
      </nav>
      <div className={styles.main}>
        <Outlet />
      </div>
    </div>
  );
}

export function AccountLayout() {
  return <SettingsShell title="My settings" links={[{ to: "/account", end: true, label: "Profile" }]} />;
}

export function OrganizationLayout() {
  const { org } = useWorkspace();

  return (
    <SettingsShell
      title={org ? org.name : "Organization"}
      links={[
        { to: "/organization", end: true, label: "Members" },
        { to: "/organization/secrets", label: "Secrets" },
        { to: "/organization/integrations", label: "Integrations" },
        { to: "/organization/webhooks", label: "Webhooks" },
        { to: "/organization/notifications", label: "Notifications" },
      ]}
    />
  );
}

export function AdminLayout() {
  return (
    <SettingsShell
      title="Instance"
      links={[
        { to: "/admin", end: true, label: "Instance" },
        { to: "/admin/authentication", label: "Authentication" },
        { to: "/admin/users", label: "Users" },
      ]}
    />
  );
}
