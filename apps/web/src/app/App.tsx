import { useEffect, useMemo, useState } from "react";
import { Button, StatusBadge } from "@shipyard/ui";
import { IconMoon, IconRefresh, IconSun } from "@tabler/icons-react";
import { AppShell } from "../shell/AppShell";

type Theme = "light" | "dark" | "system";
type HealthState = "loading" | "ok" | "error";

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") {
    return theme;
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function App() {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem("shipyard.theme");
    return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
  });
  const [health, setHealth] = useState<HealthState>("loading");
  const [info, setInfo] = useState<string>("");

  const resolved = useMemo(() => resolveTheme(theme), [theme]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", resolved);
    localStorage.setItem("shipyard.theme", theme);
  }, [resolved, theme]);

  async function refreshHealth() {
    setHealth("loading");
    try {
      const [healthRes, infoRes] = await Promise.all([
        fetch("/healthz"),
        fetch("/api/v1/system/info"),
      ]);
      if (!healthRes.ok) {
        throw new Error("health failed");
      }
      const payload = infoRes.ok ? await infoRes.json() : null;
      setInfo(payload ? `${payload.product} · ${payload.phase}` : "control plane reachable");
      setHealth("ok");
    } catch {
      setInfo("Control plane unreachable");
      setHealth("error");
    }
  }

  useEffect(() => {
    void refreshHealth();
  }, []);

  const status =
    health === "ok" ? "success" : health === "error" ? "danger" : "neutral";

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
      <section>
        <h1>Control plane</h1>
        <p>Phase 0 foundation shell. Verify API health and design tokens before identity work.</p>
        <div className="toolbar">
          <StatusBadge status={status}>
            {health === "loading" ? "Checking" : health === "ok" ? "Healthy" : "Disconnected"}
          </StatusBadge>
          <Button variant="secondary" icon={<IconRefresh size={16} />} onClick={() => void refreshHealth()} loading={health === "loading"}>
            Refresh
          </Button>
        </div>
        <p className="meta">{info || "—"}</p>
      </section>
    </AppShell>
  );
}
