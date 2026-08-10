import type { ReactNode } from "react";
import styles from "./DataTable.module.css";

export function DataTable({ children }: { children: ReactNode }) {
  return (
    <div className={styles.wrap}>
      <table className={styles.table}>{children}</table>
    </div>
  );
}
