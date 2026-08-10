import { useEffect, useMemo, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Button, StatusBadge } from "@shipyard/ui";
import { IconMoon, IconSun } from "@tabler/icons-react";
import { AppShell } from "../shell/AppShell";
import { api, User } from "./api";
import { WorkspaceProvider } from "./context/WorkspaceContext";
import { LoginPage } from "./LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { PipelinesPage } from "./pages/PipelinesPage";
import { RunDetailPage } from "./pages/RunDetailPage";
import { ArtifactsPage } from "./pages/ArtifactsPage";
import { RegistryPage } from "./pages/RegistryPage";
import { ReleasesPage } from "./pages/ReleasesPage";
import { DeploymentsPage } from "./pages/DeploymentsPage";
import { RunnersPage } from "./pages/RunnersPage";
import { ClusterPage } from "./pages/ClusterPage";
import { SettingsPage } from "./pages/SettingsPage";

type Theme = "light" | "dark" | "system";
type BootState = "loading" | "anon" | "authed" | "offline";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false },
  },
});

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function AuthedApp({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem("shipyard.theme");
    return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
  });
  const resolved = useMemo(() => resolveTheme(theme), [theme]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", resolved);
    localStorage.setItem("shipyard.theme", theme);
  }, [resolved, theme]);

  return (
    <WorkspaceProvider user={user}>
      <AppShell
        onLogout={onLogout}
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
        <Routes>
          <Route path="/" element={<OverviewPage />} />
          <Route path="/projects" element={<ProjectsPage />} />
          <Route path="/pipelines" element={<PipelinesPage />} />
          <Route path="/pipelines/runs/:runId" element={<RunDetailPage />} />
          <Route path="/artifacts" element={<ArtifactsPage />} />
          <Route path="/registry" element={<RegistryPage />} />
          <Route path="/releases" element={<ReleasesPage />} />
          <Route path="/deployments" element={<DeploymentsPage />} />
          <Route path="/runners" element={<RunnersPage />} />
          <Route path="/cluster" element={<ClusterPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </WorkspaceProvider>
  );
}

export function App() {
  const [boot, setBoot] = useState<BootState>("loading");
  const [user, setUser] = useState<User | null>(null);
  const [allowRegister, setAllowRegister] = useState(false);

  async function bootstrap() {
    setBoot("loading");
    try {
      const info = await api.systemInfo();
      setAllowRegister(Boolean(info.allow_register));
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

  if (boot === "loading") {
    return (
      <div style={{ padding: 24 }}>
        <StatusBadge status="info">Loading control plane…</StatusBadge>
      </div>
    );
  }

  if (boot === "offline") {
    return (
      <div style={{ padding: 24 }}>
        <h1>Control plane offline</h1>
        <p>Start shipyard-server and PostgreSQL, then refresh.</p>
        <Button variant="primary" onClick={() => void bootstrap()}>
          Retry
        </Button>
      </div>
    );
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
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        {user ? <AuthedApp user={user} onLogout={() => void logout()} /> : null}
      </BrowserRouter>
    </QueryClientProvider>
  );
}
