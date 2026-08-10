import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { IconBell } from "@tabler/icons-react";
import { api, AppNotification } from "../api";
import styles from "./NotificationBell.module.css";

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<AppNotification[]>([]);
  const [count, setCount] = useState(0);
  const root = useRef<HTMLDivElement>(null);

  async function refresh() {
    try {
      const res = await api.listNotifications(true);
      setItems((res.notifications ?? []).slice(0, 8));
      setCount(res.unread_count ?? 0);
    } catch {
      /* ignore while offline */
    }
  }

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), 20000);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  return (
    <div className={styles.wrap} ref={root}>
      <button
        type="button"
        className={styles.button}
        aria-label="Notifications"
        aria-expanded={open}
        onClick={() => {
          setOpen((v) => !v);
          void refresh();
        }}
      >
        <IconBell size={16} stroke={1.6} />
        {count > 0 ? <span className={styles.badge}>{count > 9 ? "9+" : count}</span> : null}
      </button>
      {open ? (
        <div className={styles.panel} role="dialog" aria-label="Notifications">
          <div className={styles.head}>
            <strong>Notifications</strong>
            <Link to="/settings/notifications" onClick={() => setOpen(false)}>
              View all
            </Link>
          </div>
          {items.length === 0 ? <div className={styles.empty}>No unread notifications</div> : null}
          <ul className={styles.list}>
            {items.map((n) => (
              <li key={n.id}>
                <Link
                  to={n.href || "/settings/notifications"}
                  className={styles.item}
                  onClick={() => {
                    void api.markNotificationRead(n.id);
                    setOpen(false);
                  }}
                >
                  <strong>{n.title}</strong>
                  <span>{n.body}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
