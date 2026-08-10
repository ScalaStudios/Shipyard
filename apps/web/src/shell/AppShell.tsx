import type { ReactNode } from "react";
import styles from "./AppShell.module.css";

type Theme = "light" | "dark" | "system";

const nav = [
  { id: "overview", label: "Overview", active: true },
  { id: "projects", label: "Projects", active: false },
  { id: "pipelines", label: "Pipelines", active: false },
  { id: "artifacts", label: "Artifacts", active: false },
  { id: "runners", label: "Runners", active: false },
];

export function AppShell({
  children,
  themeToggle,
}: {
  children: ReactNode;
  theme: Theme;
  onThemeChange: (theme: Theme) => void;
  themeToggle: ReactNode;
}) {
  return (
    <div className={styles.shell}>
      <aside className={styles.nav} aria-label="Primary">
        <div className={styles.brand}>
          <span className={styles.mark} aria-hidden="true" />
          <div>
            <strong>Shipyard</strong>
            <div className={styles.brandMeta}>Delivery control plane</div>
          </div>
        </div>
        <nav className={styles.navList}>
          {nav.map((item) => (
            <a key={item.id} className={item.active ? styles.navActive : styles.navItem} href={`#${item.id}`}>
              {item.label}
            </a>
          ))}
        </nav>
      </aside>
      <div className={styles.main}>
        <header className={styles.topbar}>
          <div>
            <div className={styles.eyebrow}>Standalone · Foundation</div>
            <div className={styles.title}>Overview</div>
          </div>
          <div className={styles.actions}>{themeToggle}</div>
        </header>
        <main className={styles.content}>{children}</main>
      </div>
    </div>
  );
}
