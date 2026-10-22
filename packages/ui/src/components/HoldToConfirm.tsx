import type { ReactNode } from "react";
import HoldButton from "../reactbits/HoldButton";

export function HoldToConfirm({
  children,
  doneLabel,
  icon,
  onConfirm,
  size = "md",
  disabled,
}: {
  children: ReactNode;
  doneLabel: ReactNode;
  icon?: ReactNode;
  onConfirm: () => void;
  size?: "sm" | "md";
  disabled?: boolean;
}) {
  return (
    <HoldButton
      size={size}
      icon={icon}
      doneLabel={doneLabel}
      onHold={onConfirm}
      disabled={disabled}
      holdTime={1200}
      releaseTime={220}
      resetAfter={0}
      radius={8}
      wave={false}
      glow={false}
      pressScale={1}
      backgroundColor="var(--sy-color-status-danger-container)"
      textColor="var(--sy-color-status-danger)"
      fillColor="var(--sy-color-danger-fill)"
      fillTextColor="#ffffff"
    >
      {children}
    </HoldButton>
  );
}
