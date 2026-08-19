import { FormEvent, useEffect, useMemo, useState } from "react";
import { Button, Panel, StatusBadge } from "@shipyard/ui";
import table from "../../components/DataTable.module.css";
import { PageHeader } from "../../components/PageHeader";
import { useWorkspace } from "../../context/WorkspaceContext";
import { api, InstanceSetting } from "../../api";
import { FormField, FormFooter, FormNote, FormSection, formStyles as form } from "./SettingsForm";

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
    hint: "Where people reach the Shipyard UI. Used for links in webhook comments and OAuth redirects back to the app.",
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
    label: "Self-registration",
    hint: "When disabled, only existing users can sign in. The very first account can always be created.",
    boolean: true,
  },
  {
    key: "webhook_secret",
    label: "Default webhook secret",
    hint: "Fallback secret used to verify incoming forge webhooks when a connection has none of its own.",
    secret: true,
    placeholder: "A long random string",
  },
];

export function InstanceSettingsPage() {
  const { setError } = useWorkspace();
  const [stored, setStored] = useState<Record<string, InstanceSetting>>({});
  const [values, setValues] = useState<Record<string, string>>({});
  const [envDefaults, setEnvDefaults] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [savedAt, setSavedAt] = useState(0);

  async function refresh() {
    const res = await api.listInstanceSettings();
    const map: Record<string, InstanceSetting> = {};
    const next: Record<string, string> = {};
    for (const item of res.settings ?? []) {
      map[item.key] = item;
      next[item.key] = item.is_secret ? "" : item.value;
    }
    for (const field of FIELDS) {
      if (next[field.key] === undefined) next[field.key] = "";
    }
    setStored(map);
    setValues(next);
  }

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "failed to load settings"));
  }, []);

  useEffect(() => {
    void api
      .systemInfo()
      .then((info) => setEnvDefaults({ allow_register: info.allow_register ? "true" : "false" }))
      .catch(() => undefined);
  }, []);

  const dirty = useMemo(
    () =>
      FIELDS.filter((field) => {
        const current = values[field.key] ?? "";
        if (field.secret) return current !== "";
        return current !== (stored[field.key]?.value ?? "");
      }),
    [values, stored],
  );

  async function save(event: FormEvent) {
    event.preventDefault();
    if (dirty.length === 0) return;
    setBusy(true);
    try {
      for (const field of dirty) {
        await api.setInstanceSetting({
          key: field.key,
          value: values[field.key] ?? "",
          is_secret: field.secret ?? false,
        });
      }
      await refresh();
      setSavedAt(Date.now());
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save settings");
    } finally {
      setBusy(false);
    }
  }

  const configured = FIELDS.filter((f) => stored[f.key]?.has_value).length;

  return (
    <div className={table.stack}>
      <PageHeader
        title="Instance"
        description="Configuration for this Shipyard instance, stored in the database. A value set here overrides the matching environment variable."
      />

      <form onSubmit={save}>
        <Panel
          title="General"
          meta={
            <StatusBadge status={configured > 0 ? "success" : "neutral"}>
              {configured} of {FIELDS.length} set
            </StatusBadge>
          }
        >
          <FormSection>
            {FIELDS.map((field) => (
              <FormField
                key={field.key}
                label={field.label}
                hint={field.hint}
                htmlFor={`setting-${field.key}`}
                source={stored[field.key]?.has_value ? undefined : "Using the environment value"}
              >
                {field.boolean ? (
                  <select
                    id={`setting-${field.key}`}
                    className={`${table.select} ${form.narrow}`}
                    value={values[field.key] || envDefaults[field.key] || "false"}
                    onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
                  >
                    <option value="true">Enabled</option>
                    <option value="false">Disabled</option>
                  </select>
                ) : (
                  <input
                    id={`setting-${field.key}`}
                    className={table.input}
                    type={field.secret ? "password" : "text"}
                    placeholder={
                      field.secret && stored[field.key]?.has_value ? "Stored — type to replace" : field.placeholder
                    }
                    value={values[field.key] ?? ""}
                    onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
                    autoComplete="off"
                    spellCheck={false}
                  />
                )}
              </FormField>
            ))}
          </FormSection>

          <FormFooter
            note={
              dirty.length > 0
                ? `${dirty.length} unsaved ${dirty.length === 1 ? "change" : "changes"}`
                : savedAt
                  ? "Saved. Changes apply immediately — no restart needed."
                  : "Changes apply immediately — no restart needed."
            }
          >
            <Button type="submit" variant="primary" loading={busy} disabled={dirty.length === 0}>
              Save changes
            </Button>
          </FormFooter>
        </Panel>
      </form>

      <Panel title="Environment only">
        <FormNote>
          <code>SHIPYARD_DATABASE_URL</code> and <code>SHIPYARD_SECRETS_KEY</code> cannot be moved here — Shipyard needs
          both before it can read or decrypt anything stored in the database. Generate a key with{" "}
          <code>head -c 32 /dev/urandom | base64</code>. The storage backend and listen address are also environment
          only.
        </FormNote>
      </Panel>
    </div>
  );
}
