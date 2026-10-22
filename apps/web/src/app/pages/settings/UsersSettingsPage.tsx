import { FormEvent, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { FormField, formStyles as form } from "../../components/FormField";
import { PageHeader } from "../../components/PageHeader";
import { TableSkeleton } from "../../components/TableSkeleton";
import { UserIdentity } from "../../components/UserIdentity";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, User } from "../../api";
import { formatTime } from "../../lib/format";

export function UsersSettingsPage() {
  const navigate = useNavigate();
  const { user, setError } = useWorkspace();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return users;
    return users.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q) ||
        (u.display_name ?? "").toLowerCase().includes(q),
    );
  }, [users, query]);

  async function refresh() {
    const res = await api.listUsers();
    setUsers(res.users ?? []);
  }

  useEffect(() => {
    void refresh()
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load users"))
      .finally(() => setLoading(false));
  }, []);

  async function createUser(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await api.createUser({ username, email, display_name: displayName, password, is_admin: isAdmin });
      setUsername("");
      setEmail("");
      setDisplayName("");
      setPassword("");
      setIsAdmin(false);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create user");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader title="Users" description="Everyone with an account on this instance." />
      <Panel title="All users" meta={<StatusBadge status="info">{users.length}</StatusBadge>}>
        <div className={table.toolbar}>
          <input
            className={table.input}
            placeholder="Search users…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            aria-label="Search users"
          />
          <span className={table.muted}>
            {filtered.length === users.length ? `${users.length} users` : `${filtered.length} of ${users.length}`}
          </span>
        </div>

        {loading ? (
          <TableSkeleton />
        ) : filtered.length === 0 ? (
          <EmptyState title="No matching users" description="No username, email, or display name matches that search." />
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>User</th>
                <th>Status</th>
                <th>Created</th>
                <th className={table.actionCol} aria-label="Manage" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((u) => {
                const self = u.id === user.id;
                return (
                  <tr key={u.id} className={table.clickRow} onClick={() => navigate(`/admin/users/${u.id}`)}>
                    <td>
                      <UserIdentity
                        name={u.display_name || u.username}
                        meta={`@${u.username} · ${u.email}`}
                        userID={u.id}
                        to={`/admin/users/${u.id}`}
                      />
                    </td>
                    <td>
                      <div className={table.formRow}>
                        {u.is_admin ? <StatusBadge status="info">Admin</StatusBadge> : null}
                        {!u.is_active ? <StatusBadge status="warning">Deactivated</StatusBadge> : null}
                        {self ? <StatusBadge status="neutral">You</StatusBadge> : null}
                      </div>
                    </td>
                    <td className={table.muted}>{formatTime(u.created_at)}</td>
                    <td className={`${table.actionCol} ${table.muted}`} aria-hidden>
                      ›
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </DataTable>
        )}
      </Panel>

      <Panel title="Create user">
        <form className={form.row} onSubmit={createUser}>
          <FormField label="Username" htmlFor="new-user-username">
            <input
              id="new-user-username"
              className={table.input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </FormField>
          <FormField label="Email" htmlFor="new-user-email">
            <input
              id="new-user-email"
              type="email"
              className={table.input}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </FormField>
          <FormField label="Display name" htmlFor="new-user-display" hint="Optional — defaults to the username.">
            <input
              id="new-user-display"
              className={table.input}
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </FormField>
          <FormField label="Password" htmlFor="new-user-password">
            <input
              id="new-user-password"
              type="password"
              className={table.input}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </FormField>
          <FormField label="Administrator" htmlFor="new-user-admin" hint="Full access to instance settings and users.">
            <span className={form.check}>
              <input
                id="new-user-admin"
                type="checkbox"
                checked={isAdmin}
                onChange={(e) => setIsAdmin(e.target.checked)}
              />
            </span>
          </FormField>
          <div className={form.action}>
            <Button type="submit" variant="primary" loading={busy}>
              Create user
            </Button>
          </div>
        </form>
      </Panel>
    </div>
  );
}
