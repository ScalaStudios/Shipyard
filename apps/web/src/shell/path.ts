import { NAV_ITEMS, type NavItem } from "./nav";

export type PathSegment = {
  label: string;
  to?: string;
};

const PROJECT_PATH_SECTIONS = new Set([
  "overview",
  "pipelines",
  "artifacts",
  "registry",
  "releases",
  "deployments",
]);

export function sectionFromPath(pathname: string): NavItem | undefined {
  const ranked = [...NAV_ITEMS].sort((a, b) => b.to.length - a.to.length);
  for (const item of ranked) {
    if (item.to === "/") {
      if (pathname === "/") return item;
      continue;
    }
    if (pathname === item.to || pathname.startsWith(`${item.to}/`)) {
      return item;
    }
  }
  return undefined;
}

export function buildPathSegments({
  orgSlug,
  projectSlug,
  pathname,
}: {
  orgSlug?: string;
  projectSlug?: string;
  pathname: string;
}): PathSegment[] {
  const section = sectionFromPath(pathname);
  const segments: PathSegment[] = [];

  segments.push({
    label: orgSlug || "org",
    to: orgSlug ? "/projects" : undefined,
  });
  if (section) {
    segments.push({ label: section.id, to: section.to });
  } else {
    const first = pathname.split("/").filter(Boolean)[0];
    if (first) segments.push({ label: first.replace(/-/g, " "), to: `/${first}` });
  }

  if (projectSlug && section && PROJECT_PATH_SECTIONS.has(section.id)) {
    segments.push({
      label: projectSlug,
      to: "/",
    });
  }

  const runMatch = pathname.match(/^\/pipelines\/runs\/([^/]+)/);
  if (runMatch) {
    segments.push({ label: runMatch[1] });
  }

  const settingsMatch = pathname.match(/^\/settings(?:\/([^/]+))?/);
  if (settingsMatch && settingsMatch[1]) {
    segments.push({ label: settingsMatch[1], to: `/settings/${settingsMatch[1]}` });
  }

  return segments;
}

