import { FormEvent, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, EmptyState, Panel, StatusBadge } from "@shipyard/ui";
import { AvatarControls } from "../../components/AvatarControls";
import table from "../../components/DataTable.module.css";
import { FormField, formStyles as form } from "../../components/FormField";
import { PageHeader } from "../../components/PageHeader";
import { TableSkeleton } from "../../components/TableSkeleton";
import { UserIdentity } from "../../components/UserIdentity";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, User } from "../../api";
import { usePageTitle } from "../../lib/usePageTitle";
import styles from "./UserDetailPage.module.css";

export function UserDetailPage() {
  const { userId = "" } = useParams();
  const { user: me, setError } = useWorkspace();
  const [target, setTarget] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [missing, setMissing] = useState(false);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [avatarVersion, setAvatarVersion] = useState(0);
  const [hasAvatar, setHasAvatar] = useState(false);
  const [confirmDeactivate, setConfirmDeactivate] = useState(false);
  const [busy, setBusy] = useState(false);

  usePageTitle(target ? target.username : "User");

  async function refresh() {
    const res = await api.getUser(userId);
    setTarget(res.user);
    setUsername(res.user.username);
    setEmail(res.user.email);
    setDisplayName(res.user.display_name);
  }

  useEffect(() => {
    setLoading(true);
    setMissing(false);
    void refresh()
      .catch(() => setMissing(true))
      .finally(() => setLoading(false));
  }, [userId]);

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await api.updateUser(userId, { username, email, display_name: displayName });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save profile");
    } finally {
      setBusy(false);
    }
  }

  async function savePassword(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await api.setUserPassword(userId, password);
      setPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to set password");
    } finally {
      setBusy(false);
    }
  }

  async function toggleAdmin() {
    if (!target) return;
    setBusy(true);
    try {
      await api.setInstanceAdmin(target.id, !target.is_admin);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update admin access");
    } finally {
      setBusy(false);
    }
  }

  async function setActive(active: boolean) {
    if (!target) return;
    if (!active && !confirmDeactivate) {
      setConfirmDeactivate(true);
      return;
    }
    setBusy(true);
    try {
      await api.setUserActive(target.id, active);
      setConfirmDeactivate(false);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update account status");
    } finally {
      setBusy(false);
    }
  }

  const self = target ? target.id === me.id : false;

  return (
    <div className={table.stack}>
      <PageHeader
        title={target ? target.display_name || target.username : "User"}
        description="Profile, avatar, password, and instance access for this account."
        actions={
          missing ? null : (
            <Link to="/admin/users">
              <Button variant="secondary">Back to users</Button>
            </Link>
          )
        }
      />

      {loading ? (
        <Panel title="Profile">
          <TableSkeleton />
        </Panel>
      ) : missing || !target ? (
        <EmptyState
          title="User not found"
          description="This account no longer exists on this instance."
          action={
            <Link to="/admin/users">
              <Button variant="secondary">Back to users</Button>
            </Link>
          }
        />
      ) : (
        <>
          <Panel>
            <div className={table.identityRow}>
              <UserIdentity
                name={target.display_name || target.username}
                meta={`@${target.username} · ${target.email}`}
                userID={target.id}
                size="lg"
                version={avatarVersion}
                onStatus={setHasAvatar}
              />
              <div className={styles.badges}>
                {target.is_admin ? <StatusBadge status="info">Admin</StatusBadge> : null}
                {!target.is_active ? <StatusBadge status="warning">Deactivated</StatusBadge> : null}
                {self ? <StatusBadge status="neutral">You</StatusBadge> : null}
              </div>
            </div>
          </Panel>

          <Panel title="Profile">
            <form className={form.row} onSubmit={saveProfile}>
              <FormField label="Username" htmlFor="user-username">
                <input
                  id="user-username"
                  className={table.input}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </FormField>
              <FormField label="Display name" htmlFor="user-display" hint="Optional — defaults to the username.">
                <input
                  id="user-display"
                  className={table.input}
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                />
              </FormField>
              <FormField label="Email" htmlFor="user-email">
                <input
                  id="user-email"
                  type="email"
                  className={table.input}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </FormField>
              <div className={form.action}>
                <Button type="submit" variant="primary" loading={busy}>
                  Save profile
                </Button>
              </div>
            </form>
          </Panel>

          <Panel title="Avatar">
            <div className={form.row}>
              <FormField label="Profile image" hint="Square PNG or JPEG, up to 2 MB.">
                <AvatarControls
                  hasAvatar={hasAvatar}
                  upload={(file) => api.uploadUserAvatar(target.id, file)}
                  remove={() => api.removeUserAvatar(target.id)}
                  onChanged={() => setAvatarVersion((v) => v + 1)}
                />
              </FormField>
            </div>
          </Panel>

          <Panel title="Security">
            <form className={form.row} onSubmit={savePassword}>
              <FormField
                label="New password"
                htmlFor="user-password"
                hint="At least 8 characters. Setting a password signs the user out everywhere."
              >
                <input
                  id="user-password"
                  type="password"
                  minLength={8}
                  className={table.input}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </FormField>
              <div className={form.action}>
                <Button type="submit" variant="secondary" loading={busy}>
                  Set password
                </Button>
              </div>
            </form>
          </Panel>

          {self ? null : (
            <Panel title="Access">
              <div className={styles.actions}>
                <Button
                  type="button"
                  variant="secondary"
                  size="compact"
                  disabled={busy}
                  onClick={() => void toggleAdmin()}
                >
                  {target.is_admin ? "Revoke admin" : "Make admin"}
                </Button>
                {target.is_active ? (
                  <Button
                    type="button"
                    variant={confirmDeactivate ? "destructive" : "secondary"}
                    size="compact"
                    disabled={busy}
                    onClick={() => void setActive(false)}
                  >
                    {confirmDeactivate ? "Confirm deactivate" : "Deactivate"}
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="secondary"
                    size="compact"
                    disabled={busy}
                    onClick={() => void setActive(true)}
                  >
                    Reactivate
                  </Button>
                )}
              </div>
            </Panel>
          )}
        </>
      )}
    </div>
  );
}
