import { useEffect, useState } from "react";
import { api } from "../api";
import styles from "./Avatar.module.css";

export function Avatar({
  name,
  userID,
  size = "md",
  version = 0,
  onStatus,
}: {
  name: string;
  userID?: string;
  size?: "sm" | "md" | "lg";
  version?: number;
  onStatus?: (hasImage: boolean) => void;
}) {
  const [broken, setBroken] = useState(false);

  useEffect(() => {
    setBroken(false);
  }, [userID, version]);

  if (!userID || broken) {
    return (
      <span className={styles.avatar} data-size={size} aria-hidden="true">
        {name.trim().slice(0, 1).toUpperCase() || "?"}
      </span>
    );
  }

  return (
    <img
      className={styles.avatar}
      data-size={size}
      src={api.avatarURL(userID, version)}
      alt=""
      onLoad={() => onStatus?.(true)}
      onError={() => {
        setBroken(true);
        onStatus?.(false);
      }}
    />
  );
}
