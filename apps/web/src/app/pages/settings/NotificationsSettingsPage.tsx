import { FormEvent, useEffect, useState } from "react";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { Link } from "react-router-dom";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, AppNotification, DiscordIntegration } from "../../api";
import { formatTime } from "../../lib/format";

export function NotificationsSettingsPage() {
  const { org, project, setError } = useWorkspace();
  const [items, setItems] = useState<AppNotification[]>([]);
  const [unread, setUnread] = useState(0);
  const [discord, setDiscord] = useState<DiscordIntegration[]>([]);
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<"webhook" | "bot">("webhook");
  const [name, setName] = useState("alerts");
  const [webhookURL, setWebhookURL] = useState("");
  const [botToken, setBotToken] = useState("");
  const [channelID, setChannelID] = useState("");

  async function refreshInbox() {
    const res = await api.listNotifications();
    setItems(res.notifications ?? []);
    setUnread(res.unread_count ?? 0);
  }

  async function refreshDiscord() {
    if (!org) {
      setDiscord([]);
      return;
    }
    const res = await api.listDiscord(org.id, project?.id);
    setDiscord(res.integrations ?? []);
  }

  useEffect(() => {
    void refreshInbox().catch((err) => setError(err instanceof Error ? err.message : "failed to load notifications"));
  }, [setError]);

  useEffect(() => {
    void refreshDiscord().catch((err) => setError(err instanceof Error ? err.message : "failed to load Discord"));
  }, [org?.id, project?.id, setError]);

  async function markAll() {
    setBusy(true);
    try {
      await api.markAllNotificationsRead();
      await refreshInbox();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to mark read");
    } finally {
      setBusy(false);
    }
  }

  async function markOne(id: string) {
    await api.markNotificationRead(id);
    await refreshInbox();
  }

  async function createDiscord(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      await api.createDiscord(org.id, {
        name,
        mode,
        webhook_url: mode === "webhook" ? webhookURL : undefined,
        bot_token: mode === "bot" ? botToken : undefined,
        channel_id: mode === "bot" ? channelID : undefined,
        project_id: project?.id,
        notify_on: ["run.started", "run.succeeded", "run.failed", "run.canceled"],
      });
      setWebhookURL("");
      setBotToken("");
      setChannelID("");
      await refreshDiscord();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to connect Discord");
    } finally {
      setBusy(false);
    }
  }

  async function testDiscord(id: string) {
    if (!org) return;
    setBusy(true);
    try {
      await api.testDiscord(org.id, id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Discord test failed");
    } finally {
      setBusy(false);
    }
  }

  async function removeDiscord(id: string) {
    if (!org) return;
    setBusy(true);
    try {
      await api.deleteDiscord(org.id, id);
      await refreshDiscord();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to remove Discord");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Notifications"
        description="In-app inbox plus Discord channel alerts — rich embeds when builds start, succeed, or fail (Sentry-style)."
        actions={
          <Button variant="secondary" loading={busy} onClick={() => void markAll()} disabled={unread === 0}>
            Mark all read
          </Button>
        }
      />

      <Panel
        title="Discord"
        meta={<StatusBadge status={discord.length ? "success" : "neutral"}>{discord.length} connected</StatusBadge>}
      >
        <div className={table.toolbar}>
          <p className={table.muted} style={{ margin: 0, maxWidth: "62ch" }}>
            Use an incoming webhook (fastest) or a Discord bot token + channel ID. Shipyard posts color-coded embeds with run links.
          </p>
        </div>
        {!org ? (
          <EmptyState title="Select an organization" />
        ) : (
          <form className={table.toolbar} onSubmit={createDiscord} style={{ flexDirection: "column", alignItems: "stretch" }}>
            <div className={table.formRow}>
              <select className={table.select} value={mode} onChange={(e) => setMode(e.target.value as "webhook" | "bot")} aria-label="Mode">
                <option value="webhook">Incoming webhook</option>
                <option value="bot">Bot token</option>
              </select>
              <input className={table.input} placeholder="name" value={name} onChange={(e) => setName(e.target.value)} required pattern="[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?" />
              {mode === "webhook" ? (
                <input
                  className={table.input}
                  style={{ minWidth: 280 }}
                  placeholder="https://discord.com/api/webhooks/…"
                  value={webhookURL}
                  onChange={(e) => setWebhookURL(e.target.value)}
                  required
                />
              ) : (
                <>
                  <input className={table.input} type="password" placeholder="bot token" value={botToken} onChange={(e) => setBotToken(e.target.value)} required />
                  <input className={table.input} placeholder="channel id" value={channelID} onChange={(e) => setChannelID(e.target.value)} required />
                </>
              )}
              <Button type="submit" variant="primary" loading={busy}>
                Connect Discord
              </Button>
            </div>
          </form>
        )}
        {discord.length === 0 ? (
          <EmptyState title="No Discord channels" description="Create a webhook in Discord → Channel settings → Integrations." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Mode</th>
                <th>Events</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {discord.map((d) => (
                <tr key={d.id}>
                  <td>{d.name}</td>
                  <td className="mono">{d.mode}</td>
                  <td className={table.muted}>{(d.notify_on ?? []).join(", ")}</td>
                  <td>
                    <Button variant="secondary" disabled={busy} onClick={() => void testDiscord(d.id)}>
                      Send test
                    </Button>{" "}
                    <Button variant="ghost" disabled={busy} onClick={() => void removeDiscord(d.id)}>
                      Remove
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Panel>

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
