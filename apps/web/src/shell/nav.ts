import type { LucideIcon } from "lucide-react";
import {
  Building2,
  Container,
  Folder,
  GitBranch,
  LayoutDashboard,
  Network,
  Rocket,
  Server,
  ShieldCheck,
  Warehouse,
} from "lucide-react";

export type NavItem = {
  id: string;
  label: string;
  to: string;
  icon: LucideIcon;
  summary: string;
  adminOnly?: boolean;
};

export type NavGroup = { id: string; label: string | null; items: NavItem[] };

export const NAV_GROUPS: NavGroup[] = [
  {
    id: "home",
    label: null,
    items: [
      { id: "overview", label: "Overview", to: "/", icon: LayoutDashboard, summary: "Delivery health and what needs attention." },
      { id: "projects", label: "Projects", to: "/projects", icon: Folder, summary: "Organizations, projects and forge imports." },
    ],
  },
  {
    id: "delivery",
    label: "Delivery",
    items: [
      { id: "pipelines", label: "Pipelines", to: "/pipelines", icon: GitBranch, summary: "Pipeline definitions, runs and job logs." },
      { id: "registry", label: "Registry", to: "/registry", icon: Container, summary: "Artifacts, OCI images and Maven or npm packages." },
      { id: "releases", label: "Releases", to: "/releases", icon: Rocket, summary: "Versioned bundles of artifacts and images." },
      { id: "deployments", label: "Deployments", to: "/deployments", icon: Warehouse, summary: "Rollouts to environments and their history." },
    ],
  },
  {
    id: "infrastructure",
    label: "Infrastructure",
    items: [
      { id: "runners", label: "Runners", to: "/runners", icon: Server, summary: "Build agents, heartbeats and install tokens." },
      { id: "cluster", label: "Cluster", to: "/cluster", icon: Network, summary: "Control-plane nodes and their state." },
    ],
  },
  {
    id: "administration",
    label: "Administration",
    items: [
      { id: "organization", label: "Organization", to: "/organization", icon: Building2, summary: "Members, secrets, integrations and webhooks." },
      { id: "admin", label: "Instance", to: "/admin", icon: ShieldCheck, summary: "Instance settings, authentication and users.", adminOnly: true },
    ],
  },
];

export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((group) => group.items);
