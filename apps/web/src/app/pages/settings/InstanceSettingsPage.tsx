import { FormEvent, useEffect, useState } from "react";
import { Button, Panel, StatusBadge } from "@shipyard/ui";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, InstanceSetting } from "../../api";

type Field = {
  key: string;
  label: string;
  hint: string;
  secret?: boolean;
  boolean?: boolean;
  placeholder?: string;
};

const FIELDS: Field[] = [
  {
    key: "public_url",
    label: "Public URL",
    hint: "Where people reach the Shipyard UI. Used in webhook links and OAuth redirects back to the app.",
    placeholder: "https://shipyard.example.com",
  },
  {
    key: "api_url",
    label: "API URL",
    hint: "Origin the API is reachable on. Used in runner install scripts and provider callback URLs.",
    placeholder: "https://shipyard.example.com",
  },
  {
    key: "allow_register",
    label: "Allow self-registration",
    hint: "When off, only existing users can sign in. The first account can always be created.",
    boolean: true,
  },
  {
    key: "webhook_secret",
    label: "Default webhook secret",
    hint: "Fallback secret used to verify incoming forge webhooks when a connection has none.",
    secret: true,
  },
];

export function InstanceSettingsPage() {
  const { setError } = useWorkspace();
  const [values, setValues] = useState<Record<string, string>>({});
  const [stored, setStored] = useState<Record<string, InstanceSetting>>({});
  const [busy, setBusy] = useState("");

  async function refresh() {
    const res = await api.listInstanceSettings();
    const map: Record<string, InstanceSetting> = {};
    const next: Record<string, string> = {};
    for (const item of res.settings ?? []) {
      map[item.key] = item;
      if (!item.is_secret) next[item.key] = item.value;
    }
    setStored(map);
    setValues(next);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load settings"));
  }, []);

  async function save(field: Field, event: FormEvent) {
    event.preventDefault();
    setBusy(field.key);
    try {
      await api.setInstanceSetting({
        key: field.key,
        value: values[field.key] ?? "",
        is_secret: field.secret ?? false,
      });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save setting");
    } finally {
      setBusy("");
    }
  }

  return (
    <div className={table.stack}>
      <PageHeader
        title="Instance"
        description="Instance-wide configuration stored in the database. Values set here override the matching environment variable."
      />

      {FIELDS.map((field) => (
        <Panel
          key={field.key}
          title={field.label}
          meta={
            <StatusBadge status={stored[field.key]?.has_value ? "success" : "neutral"}>
              {stored[field.key]?.has_value ? "set" : "from environment"}
            </StatusBadge>
          }
        >
          <p className={table.muted}>{field.hint}</p>
          <form className={table.formRow} onSubmit={(e) => void save(field, e)}>
            {field.boolean ? (
              <select
                className={table.select}
                value={values[field.key] ?? "false"}
                onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
                aria-label={field.label}
              >
                <option value="true">Enabled</option>
                <option value="false">Disabled</option>
              </select>
            ) : (
              <input
                className={table.input}
                type={field.secret ? "password" : "text"}
                placeholder={field.secret && stored[field.key]?.has_value ? "stored — type to replace" : field.placeholder}
                value={values[field.key] ?? ""}
                onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
                autoComplete="off"
              />
            )}
            <Button type="submit" variant="primary" loading={busy === field.key}>
              Save
            </Button>
          </form>
        </Panel>
      ))}

      <Panel title="Bootstrap configuration">
        <p className={table.muted}>
          <code>SHIPYARD_DATABASE_URL</code> and <code>SHIPYARD_SECRETS_KEY</code> stay in the environment — Shipyard needs
          them before it can read or decrypt anything stored here. Generate a key with{" "}
          <code>head -c 32 /dev/urandom | base64</code>. Storage backend and listen address are also environment-only.
        </p>
      </Panel>
    </div>
  );
}
