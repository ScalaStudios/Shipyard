import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router-dom";
import { IconBell } from "@tabler/icons-react";
import { api, AppNotification } from "../api";
import styles from "./NotificationBell.module.css";

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<AppNotification[]>([]);
  const [count, setCount] = useState(0);
  const [panelPos, setPanelPos] = useState({ top: 48, right: 12 });
  const buttonRef = useRef<HTMLButtonElement>(null);

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
    if (!open) return;
    function place() {
      const rect = buttonRef.current?.getBoundingClientRect();
      if (!rect) return;
      setPanelPos({
        top: Math.round(rect.bottom + 8),
        right: Math.round(Math.max(12, window.innerWidth - rect.right)),
      });
    }
    function onDoc(e: MouseEvent) {
      const target = e.target as Node;
      if (buttonRef.current?.contains(target)) return;
      const panel = document.querySelector(`[data-notification-panel="true"]`);
      if (panel?.contains(target)) return;
      setOpen(false);
    }
    place();
    window.addEventListener("resize", place);
    document.addEventListener("mousedown", onDoc);
    return () => {
      window.removeEventListener("resize", place);
      document.removeEventListener("mousedown", onDoc);
    };
  }, [open]);

  const panel =
    open && typeof document !== "undefined"
      ? createPortal(
          <div
            className={styles.panel}
            role="dialog"
            aria-label="Notifications"
            data-notification-panel="true"
            style={{ top: panelPos.top, right: panelPos.right }}
          >
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
                      void api
                        .markNotificationRead(n.id)
                        .then(() => refresh())
                        .catch(() => undefined);
                      setOpen(false);
                    }}
                  >
                    <strong>{n.title}</strong>
                    <span>{n.body}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </div>,
          document.body,
        )
      : null;

  return (
    <div className={styles.wrap}>
      <button
        ref={buttonRef}
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
      {panel}
    </div>
  );
}
