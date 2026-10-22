import type { ReactNode } from "react";
import styles from "./Panel.module.css";

export function Panel({
  title,
  description,
  meta,
  actions,
  children,
  className,
}: {
  title?: string;
  description?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const classes = [styles.panel, className].filter(Boolean).join(" ");
  return (
    <section className={classes}>
      {title || meta || actions ? (
        <header className={styles.head}>
          <div className={styles.headCopy}>
            <div className={styles.headMain}>
              {title ? <h2 className={styles.title}>{title}</h2> : null}
              {meta}
            </div>
            {description ? <p className={styles.description}>{description}</p> : null}
          </div>
          {actions ? <div className={styles.actions}>{actions}</div> : null}
        </header>
      ) : null}
      <div className={styles.body}>{children}</div>
    </section>
  );
}
