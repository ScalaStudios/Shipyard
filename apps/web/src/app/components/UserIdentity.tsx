import { Link } from "react-router-dom";
import { Avatar } from "./Avatar";
import styles from "./UserIdentity.module.css";

export function UserIdentity({
  name,
  meta,
  userID,
  to,
  size,
  version,
  onStatus,
}: {
  name: string;
  meta: string;
  userID?: string;
  to?: string;
  size?: "sm" | "md" | "lg";
  version?: number;
  onStatus?: (hasImage: boolean) => void;
}) {
  return (
    <div className={styles.identity} data-size={size}>
      <Avatar name={name} userID={userID} size={size} version={version} onStatus={onStatus} />
      <span className={styles.text}>
        <span className={styles.name}>
          {to ? (
            <Link to={to} onClick={(e) => e.stopPropagation()}>
              {name}
            </Link>
          ) : (
            name
          )}
        </span>
        <span className={styles.meta}>{meta}</span>
      </span>
    </div>
  );
}
