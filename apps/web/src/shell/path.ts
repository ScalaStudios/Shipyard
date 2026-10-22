import { NAV_ITEMS, type NavItem } from "./nav";

export type PathSegment = {
  label: string;
  to?: string;
};

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
  pathname,
}: {
  orgSlug?: string;
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

  const runMatch = pathname.match(/^\/pipelines\/runs\/([^/]+)/);
  if (runMatch) {
    segments.push({ label: `run ${runMatch[1].slice(0, 8)}` });
  }

  const areaMatch = pathname.match(/^\/(organization|admin|account)(?:\/([^/]+))?/);
  if (areaMatch && areaMatch[2]) {
    segments.push({ label: areaMatch[2], to: `/${areaMatch[1]}/${areaMatch[2]}` });
  }

  return segments;
}

