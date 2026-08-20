import styles from "./TableSkeleton.module.css";

const ROWS = [0, 1, 2];

export function TableSkeleton() {
  return (
    <div className={styles.skeleton} role="status" aria-label="Loading">
      {ROWS.map((row) => (
        <div key={row} className={styles.row}>
          <span className={styles.bar} />
          <span className={styles.bar} />
          <span className={styles.bar} />
        </div>
      ))}
    </div>
  );
}
