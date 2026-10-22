import type { ButtonHTMLAttributes, ReactNode } from "react";
import styles from "./Button.module.css";

type Variant = "primary" | "secondary" | "ghost" | "destructive";

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  icon?: ReactNode;
  loading?: boolean;
};

export function Button({
  variant = "secondary",
  icon,
  loading = false,
  children,
  className,
  disabled,
  ...rest
}: ButtonProps) {
  const classes = [styles.button, styles[variant], className].filter(Boolean).join(" ");
  return (
    <button
      className={classes}
      disabled={disabled || loading}
      data-loading={loading || undefined}
      aria-busy={loading || undefined}
      {...rest}
    >
      <span className={styles.content}>
        {icon ? <span className={styles.icon}>{icon}</span> : null}
        <span>{children}</span>
      </span>
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : null}
    </button>
  );
}
