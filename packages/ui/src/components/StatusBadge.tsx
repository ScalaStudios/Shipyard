import type { ReactNode } from "react";
import styles from "./StatusBadge.module.css";

export type Status = "success" | "warning" | "danger" | "info" | "neutral";

const labels: Record<Status, string> = {
  success: "Success",
  warning: "Warning",
  danger: "Failed",
  info: "Info",
  neutral: "Unknown",
};

export function StatusBadge({ status, children }: { status: Status; children?: ReactNode }) {
  return (
    <span className={`${styles.badge} ${styles[status]}`} data-status={status}>
      <span className={styles.dot} aria-hidden="true" />
      <span>{children ?? labels[status]}</span>
    </span>
  );
}
