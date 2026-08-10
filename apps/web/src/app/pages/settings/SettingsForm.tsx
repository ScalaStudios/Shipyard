import { ReactNode } from "react";
import styles from "./SettingsForm.module.css";

export function FormSection({ children }: { children: ReactNode }) {
  return <div className={styles.section}>{children}</div>;
}

export function FormIntro({ children }: { children: ReactNode }) {
  return <p className={styles.intro}>{children}</p>;
}

export function FormField({
  label,
  hint,
  htmlFor,
  source,
  children,
}: {
  label: string;
  hint?: ReactNode;
  htmlFor?: string;
  source?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className={styles.field}>
      <div>
        <label className={styles.label} htmlFor={htmlFor}>
          {label}
        </label>
        {hint ? <p className={styles.hint}>{hint}</p> : null}
      </div>
      <div className={styles.control}>
        {children}
        {source ? <span className={styles.source}>{source}</span> : null}
      </div>
    </div>
  );
}

export function FormGrid({ children }: { children: ReactNode }) {
  return <div className={styles.grid}>{children}</div>;
}

export function GridField({
  label,
  htmlFor,
  wide,
  children,
}: {
  label: string;
  htmlFor?: string;
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={wide ? `${styles.gridField} ${styles.span2}` : styles.gridField}>
      <label className={styles.label} htmlFor={htmlFor}>
        {label}
      </label>
      {children}
    </div>
  );
}

export function FormFooter({ note, children }: { note?: ReactNode; children: ReactNode }) {
  return (
    <div className={styles.footer}>
      <span className={styles.footerNote}>{note}</span>
      {children}
    </div>
  );
}

export function FormNote({ children }: { children: ReactNode }) {
  return <p className={styles.note}>{children}</p>;
}

export const formStyles = styles;
