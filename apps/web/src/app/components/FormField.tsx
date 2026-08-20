import type { ReactNode } from "react";
import styles from "./FormField.module.css";

export function FormField({
  label,
  hint,
  htmlFor,
  children,
}: {
  label: string;
  hint?: ReactNode;
  htmlFor?: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.field}>
      <label className={styles.label} htmlFor={htmlFor}>
        {label}
      </label>
      {children}
      {hint ? <p className={styles.hint}>{hint}</p> : null}
    </div>
  );
}

export const formStyles = styles;
