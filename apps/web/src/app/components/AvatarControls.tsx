import { ChangeEvent, useRef, useState } from "react";
import { Button } from "@shipyard/ui";
import { useWorkspace } from "../context/WorkspaceContext";
import table from "./DataTable.module.css";

const MAX_AVATAR_BYTES = 2 * 1024 * 1024;

export function AvatarControls({
  hasAvatar,
  upload,
  remove,
  onChanged,
}: {
  hasAvatar: boolean;
  upload: (file: File) => Promise<unknown>;
  remove: () => Promise<unknown>;
  onChanged: () => void;
}) {
  const { setError } = useWorkspace();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  async function choose(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setError("Pick an image file.");
      return;
    }
    if (file.size > MAX_AVATAR_BYTES) {
      setError("Images must be 2 MB or smaller.");
      return;
    }
    setBusy(true);
    try {
      await upload(file);
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to upload image");
    } finally {
      setBusy(false);
    }
  }

  async function clear() {
    setBusy(true);
    try {
      await remove();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to remove image");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={table.formRow}>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        hidden
        aria-label="Profile image file"
        onChange={(e) => void choose(e)}
      />
      <Button type="button" variant="secondary" size="compact" loading={busy} onClick={() => inputRef.current?.click()}>
        Upload image
      </Button>
      {hasAvatar ? (
        <Button type="button" variant="ghost" size="compact" disabled={busy} onClick={() => void clear()}>
          Remove
        </Button>
      ) : null}
    </div>
  );
}
