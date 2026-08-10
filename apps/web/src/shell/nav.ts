import type { ComponentType } from "react";
import {
  IconBox,
  IconBuildingWarehouse,
  IconLayoutDashboard,
  IconPackages,
  IconRocket,
  IconSettings,
  IconTopologyStar3,
  IconGitBranch,
  IconServer,
  IconFolder,
} from "@tabler/icons-react";

export type NavItem = {
  id: string;
  label: string;
  to: string;
  icon: ComponentType<{ size?: number; stroke?: number }>;
};

export const NAV_ITEMS: NavItem[] = [
  { id: "overview", label: "Overview", to: "/", icon: IconLayoutDashboard },
  { id: "projects", label: "Projects", to: "/projects", icon: IconFolder },
  { id: "pipelines", label: "Pipelines", to: "/pipelines", icon: IconGitBranch },
  { id: "artifacts", label: "Artifacts", to: "/artifacts", icon: IconBox },
  { id: "registry", label: "Registry", to: "/registry", icon: IconPackages },
  { id: "releases", label: "Releases", to: "/releases", icon: IconRocket },
  { id: "deployments", label: "Deployments", to: "/deployments", icon: IconBuildingWarehouse },
  { id: "runners", label: "Runners", to: "/runners", icon: IconServer },
  { id: "cluster", label: "Cluster", to: "/cluster", icon: IconTopologyStar3 },
  { id: "settings", label: "Settings", to: "/settings", icon: IconSettings },
];
