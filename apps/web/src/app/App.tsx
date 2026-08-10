import { useEffect, useMemo, useState } from "react";
import { Button, StatusBadge } from "@shipyard/ui";
import { IconMoon, IconRefresh, IconSun } from "@tabler/icons-react";
import { AppShell } from "../shell/AppShell";
import { api, User } from "./api";
import { LoginPage } from "./LoginPage";
import { Workspace } from "./Workspace";

type Theme = "light" | "dark" | "system";
type BootState = "loading" | "anon" | "authed" | "offline";

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function App() {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem("shipyard.theme");
    return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
  });
  const [boot, setBoot] = useState<BootState>("loading");
  const [user, setUser] = useState<User | null>(null);
  const [allowRegister, setAllowRegister] = useState(false);
  const [phase, setPhase] = useState("—");

  const resolved = useMemo(() => resolveTheme(theme), [theme]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", resolved);
    localStorage.setItem("shipyard.theme", theme);
  }, [resolved, theme]);

  async function bootstrap() {
    setBoot("loading");
    try {
      const info = await api.systemInfo();
      setAllowRegister(Boolean(info.allow_register));
      setPhase(info.phase);
      try {
        const me = await api.me();
        setUser(me.user);
        setBoot("authed");
      } catch {
        setUser(null);
        setBoot("anon");
      }
    } catch {
      setUser(null);
      setBoot("offline");
    }
  }

  useEffect(() => {
    void bootstrap();
  }, []);

  async function logout() {
    await api.logout();
    setUser(null);
    setBoot("anon");
  }

  if (boot === "anon") {
    return (
      <LoginPage
        allowRegister={allowRegister}
        onAuthed={() => {
          void bootstrap();
        }}
      />
    );
  }

  return (
    <AppShell
      theme={theme}
      onThemeChange={setTheme}
      themeToggle={
        <Button
          variant="ghost"
          aria-label="Toggle theme"
          icon={resolved === "dark" ? <IconSun size={16} /> : <IconMoon size={16} />}
          onClick={() => setTheme(resolved === "dark" ? "light" : "dark")}
        >
          {resolved === "dark" ? "Light" : "Dark"}
        </Button>
      }
    >
      <div className="toolbar">
        <StatusBadge status={boot === "authed" ? "success" : boot === "offline" ? "danger" : "neutral"}>
          {boot === "authed" ? "Authenticated" : boot === "offline" ? "Disconnected" : "Loading"}
        </StatusBadge>
        <Button variant="secondary" icon={<IconRefresh size={16} />} onClick={() => void bootstrap()} loading={boot === "loading"}>
          Refresh
        </Button>
        <span className="meta">{phase}</span>
      </div>

      {boot === "offline" ? (
        <section>
          <h1>Control plane offline</h1>
          <p>Start shipyard-server and PostgreSQL, then refresh.</p>
        </section>
      ) : null}

      {boot === "authed" && user ? <Workspace user={user} onLogout={() => void logout()} /> : null}
    </AppShell>
  );
}
