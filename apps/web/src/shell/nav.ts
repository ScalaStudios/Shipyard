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

export type NavGroup = {
  label?: string;
  items: NavItem[];
};

export const NAV_GROUPS: NavGroup[] = [
  {
    items: [{ id: "overview", label: "Overview", to: "/", icon: IconLayoutDashboard }],
  },
  {
    label: "Build",
    items: [
      { id: "projects", label: "Projects", to: "/projects", icon: IconFolder },
      { id: "pipelines", label: "Pipelines", to: "/pipelines", icon: IconGitBranch },
      { id: "artifacts", label: "Artifacts", to: "/artifacts", icon: IconBox },
    ],
  },
  {
    label: "Distribute",
    items: [
      { id: "registry", label: "Registry", to: "/registry", icon: IconPackages },
      { id: "releases", label: "Releases", to: "/releases", icon: IconRocket },
      { id: "deployments", label: "Deployments", to: "/deployments", icon: IconBuildingWarehouse },
    ],
  },
  {
    label: "Infrastructure",
    items: [
      { id: "runners", label: "Runners", to: "/runners", icon: IconServer },
      { id: "cluster", label: "Cluster", to: "/cluster", icon: IconTopologyStar3 },
    ],
  },
];

export const SETTINGS_ITEM: NavItem = { id: "settings", label: "Settings", to: "/settings", icon: IconSettings };

export const NAV_ITEMS: NavItem[] = [...NAV_GROUPS.flatMap((group) => group.items), SETTINGS_ITEM];
