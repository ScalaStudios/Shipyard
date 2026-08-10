import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, type Organization, type Project, type User } from "../api";

const ORG_KEY = "shipyard.selectedOrgId";
const PROJECT_KEY = "shipyard.selectedProjectId";

type WorkspaceContextValue = {
  user: User;
  orgs: Organization[];
  projects: Project[];
  org: Organization | null;
  project: Project | null;
  loading: boolean;
  error: string;
  setOrgID: (id: string) => void;
  setProjectID: (id: string) => void;
  refreshOrgs: () => Promise<void>;
  refreshProjects: () => Promise<void>;
  clearError: () => void;
  setError: (message: string) => void;
};

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

export function WorkspaceProvider({ user, children }: { user: User; children: ReactNode }) {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [orgID, setOrgIDState] = useState(() => localStorage.getItem(ORG_KEY) ?? "");
  const [projectID, setProjectIDState] = useState(() => localStorage.getItem(PROJECT_KEY) ?? "");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const org = useMemo(() => orgs.find((o) => o.id === orgID) ?? orgs[0] ?? null, [orgs, orgID]);
  const project = useMemo(
    () => projects.find((p) => p.id === projectID) ?? projects[0] ?? null,
    [projects, projectID],
  );

  const refreshOrgs = useCallback(async () => {
    const res = await api.listOrgs();
    setOrgs(res.organizations ?? []);
  }, []);

  const refreshProjects = useCallback(async () => {
    if (!org) {
      setProjects([]);
      return;
    }
    const res = await api.listProjects(org.id);
    setProjects(res.projects ?? []);
  }, [org]);

  useEffect(() => {
    setLoading(true);
    void refreshOrgs()
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load organizations"))
      .finally(() => setLoading(false));
  }, [refreshOrgs]);

  useEffect(() => {
    if (!org) {
      setProjects([]);
      return;
    }
    localStorage.setItem(ORG_KEY, org.id);
    void refreshProjects().catch((err) => setError(err instanceof Error ? err.message : "failed to load projects"));
  }, [org, refreshProjects]);

  useEffect(() => {
    if (project) localStorage.setItem(PROJECT_KEY, project.id);
  }, [project]);

  const setOrgID = useCallback((id: string) => {
    setOrgIDState(id);
    setProjectIDState("");
    localStorage.setItem(ORG_KEY, id);
    localStorage.removeItem(PROJECT_KEY);
  }, []);

  const setProjectID = useCallback((id: string) => {
    setProjectIDState(id);
    localStorage.setItem(PROJECT_KEY, id);
  }, []);

  const value = useMemo<WorkspaceContextValue>(
    () => ({
      user,
      orgs,
      projects,
      org,
      project,
      loading,
      error,
      setOrgID,
      setProjectID,
      refreshOrgs,
      refreshProjects,
      clearError: () => setError(""),
      setError,
    }),
    [user, orgs, projects, org, project, loading, error, setOrgID, setProjectID, refreshOrgs, refreshProjects],
  );

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspace() {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) throw new Error("useWorkspace requires WorkspaceProvider");
  return ctx;
}
