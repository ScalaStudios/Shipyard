import { useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { Link } from "react-router-dom";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, AppNotification } from "../../api";
import { formatTime } from "../../lib/format";

export function NotificationsSettingsPage() {
  const { setError } = useWorkspace();
  const [items, setItems] = useState<AppNotification[]>([]);
  const [unread, setUnread] = useState(0);
  const [busy, setBusy] = useState(false);

  async function refresh() {
    const res = await api.listNotifications();
    setItems(res.notifications ?? []);
    setUnread(res.unread_count ?? 0);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load notifications"));
  }, [setError]);

  async function markAll() {
    setBusy(true);
    try {
      await api.markAllNotificationsRead();
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to mark read");
    } finally {
      setBusy(false);
    }
  }

  async function markOne(id: string) {
    await api.markNotificationRead(id);
    await refresh();
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Notifications"
        description="In-app alerts when builds start, succeed, fail, or deploy. Forge bots comment on PRs separately."
        actions={
          <Button variant="secondary" loading={busy} onClick={() => void markAll()} disabled={unread === 0}>
            Mark all read
          </Button>
        }
      />
      <Panel title="Inbox" meta={<StatusBadge status={unread ? "warning" : "success"}>{unread} unread</StatusBadge>}>
        {items.length === 0 ? (
          <EmptyState title="No notifications" description="Webhook-triggered runs will appear here." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>When</th>
                <th>Title</th>
                <th>Detail</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((n) => (
                <tr key={n.id}>
                  <td className={table.muted}>{formatTime(n.created_at)}</td>
                  <td>
                    {!n.read_at ? <StatusBadge status="info">new</StatusBadge> : null} {n.title}
                  </td>
                  <td className={table.muted}>{n.body}</td>
                  <td>
                    {n.href ? <Link to={n.href}>Open</Link> : null}{" "}
                    {!n.read_at ? (
                      <button type="button" className={table.rowButton} onClick={() => void markOne(n.id)}>
                        Mark read
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>
    </div>
  );
}
