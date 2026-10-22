import { FormEvent, useEffect, useMemo, useState } from "react";
import { Button, EmptyState, HoldToConfirm, Panel, StatusBadge } from "@shipyard/ui";
import { DataTable } from "../../components/DataTable";
import table from "../../components/DataTable.module.css";
import { FormField, formStyles as form } from "../../components/FormField";
import { PageHeader } from "../../components/PageHeader";
import { UserIdentity } from "../../components/UserIdentity";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, Member } from "../../api";
import { formatTime } from "../../lib/format";

const ROLES = ["owner", "admin", "maintainer", "developer", "viewer"];

export function MembersSettingsPage() {
  const { org, user, setError } = useWorkspace();
  const [members, setMembers] = useState<Member[]>([]);
  const [login, setLogin] = useState("");
  const [role, setRole] = useState("developer");
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const canManage = org?.role === "owner" || org?.role === "admin";

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return members;
    return members.filter(
      (m) => m.username.toLowerCase().includes(q) || (m.display_name ?? "").toLowerCase().includes(q),
    );
  }, [members, query]);

  async function refresh() {
    if (!org) {
      setMembers([]);
      return;
    }
    const res = await api.listMembers(org.id);
    setMembers(res.members ?? []);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load members"));
  }, [org?.id]);

  async function invite(event: FormEvent) {
    event.preventDefault();
    if (!org) return;
    setBusy(true);
    try {
      await api.addMember(org.id, { login, role });
      setLogin("");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to add member");
    } finally {
      setBusy(false);
    }
  }

  async function changeRole(member: Member, next: string) {
    if (!org) return;
    setBusy(true);
    try {
      await api.addMember(org.id, { login: member.username, role: next });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to change role");
    } finally {
      setBusy(false);
    }
  }

  async function remove(member: Member) {
    if (!org) return;
    setBusy(true);
    try {
      await api.removeMember(org.id, member.user_id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to remove member");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader title="Members" description="Organization membership and roles for the selected org." />
      <Panel
        title="All members"
        meta={<StatusBadge status={org ? "info" : "neutral"}>{members.length}</StatusBadge>}
      >
        {!org ? (
          <EmptyState title="Select an organization" />
        ) : members.length === 0 ? (
          <EmptyState title="No members" />
        ) : (
          <>
            <div className={table.toolbar}>
              <input
                className={table.input}
                placeholder="Search members…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                aria-label="Search members"
              />
              <span className={table.muted}>
                {filtered.length === members.length
                  ? `${members.length} members`
                  : `${filtered.length} of ${members.length}`}
              </span>
            </div>
            {filtered.length === 0 ? (
              <EmptyState title="No matching members" description="No username or display name matches that search." />
            ) : (
              <DataTable>
                <thead>
                  <tr>
                    <th>User</th>
                    <th>Role</th>
                    <th>Joined</th>
                    <th className={table.actionCol} />
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((m) => {
                    const self = m.user_id === user.id;
                    return (
                      <tr key={m.user_id}>
                        <td>
                          <UserIdentity name={m.display_name || m.username} meta={`@${m.username}`} userID={m.user_id} />
                        </td>
                        <td>
                          {canManage && !self ? (
                            <select
                              className={table.select}
                              value={m.role}
                              disabled={busy}
                              aria-label={`Role for ${m.username}`}
                              onChange={(e) => void changeRole(m, e.target.value)}
                            >
                              {ROLES.map((r) => (
                                <option key={r} value={r}>
                                  {r}
                                </option>
                              ))}
                            </select>
                          ) : (
                            <div className={table.formRow}>
                              <span>{m.role}</span>
                              {self ? <StatusBadge status="neutral">You</StatusBadge> : null}
                            </div>
                          )}
                        </td>
                        <td className={table.muted}>{formatTime(m.created_at)}</td>
                        <td className={table.actionCol}>
                          {canManage && !self ? (
                            <HoldToConfirm size="sm" doneLabel="Removed" disabled={busy} onConfirm={() => void remove(m)}>
                              Hold to remove
                            </HoldToConfirm>
                          ) : null}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </DataTable>
            )}
          </>
        )}
      </Panel>

      <Panel title="Add member">
        <form className={form.row} onSubmit={invite}>
          <FormField label="Username or email" htmlFor="member-login">
            <input
              id="member-login"
              className={table.input}
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              required
              disabled={!org}
            />
          </FormField>
          <FormField label="Role" htmlFor="member-role">
            <select
              id="member-role"
              className={table.select}
              value={role}
              onChange={(e) => setRole(e.target.value)}
              disabled={!org}
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </FormField>
          <div className={form.action}>
            <Button type="submit" variant="primary" loading={busy} disabled={!org}>
              Add member
            </Button>
          </div>
        </form>
      </Panel>
    </div>
  );
}
