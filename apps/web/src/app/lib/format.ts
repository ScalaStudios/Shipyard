import type { Status } from "@shipyard/ui";

export function runStatus(status: string): Status {
  switch (status) {
    case "succeeded":
    case "online":
    case "ready":
      return "success";
    case "failed":
    case "cancelled":
    case "offline":
      return "danger";
    case "running":
    case "queued":
    case "pending":
    case "leasing":
      return "info";
    case "warning":
    case "drained":
      return "warning";
    default:
      return "neutral";
  }
}

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "—";
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n;
  let i = -1;
  do {
    v /= 1024;
    i += 1;
  } while (v >= 1024 && i < units.length - 1);
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatTime(value?: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export const defaultPipelineYAML = `pipeline:
  name: hello
jobs:
  greet:
    runner:
      os: linux
    steps:
      - name: echo
        run: echo hello from shipyard
`;
