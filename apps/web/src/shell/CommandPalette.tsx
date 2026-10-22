import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Folder, Search } from "lucide-react";
import { useWorkspace } from "../app/context/WorkspaceContext";
import { NAV_ITEMS } from "./nav";
import styles from "./CommandPalette.module.css";

type Command = { id: string; label: string; hint: string; group: string; icon: typeof Folder; run: () => void };

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  const { user, projects, setProjectID } = useWorkspace();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      setQuery("");
      setActive(0);
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  const commands = useMemo<Command[]>(
    () => [
      ...NAV_ITEMS.filter((item) => !item.adminOnly || user.is_admin).map((item) => ({
        id: item.id,
        label: item.label,
        hint: item.summary,
        group: "Go to",
        icon: item.icon,
        run: () => navigate(item.to),
      })),
      ...projects.map((p) => ({
        id: `project-${p.id}`,
        label: p.name,
        hint: p.slug,
        group: "Switch project",
        icon: Folder,
        run: () => {
          setProjectID(p.id);
          navigate("/");
        },
      })),
    ],
    [navigate, projects, setProjectID, user.is_admin],
  );

  const needle = query.trim().toLowerCase();
  const results = needle
    ? commands.filter((c) => c.label.toLowerCase().includes(needle) || c.hint.toLowerCase().includes(needle))
    : commands;

  useEffect(() => {
    if (results[active]) document.getElementById(`command-${results[active].id}`)?.scrollIntoView({ block: "nearest" });
  }, [active, results]);

  function choose(command: Command | undefined) {
    if (!command) return;
    onClose();
    command.run();
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((i) => Math.min(i + 1, results.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(results[active]);
    }
  }

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-label="Command palette"
      onClose={onClose}
      onClick={(event) => {
        if (event.target === dialogRef.current) onClose();
      }}
    >
      <div className={styles.field}>
        <Search size={16} strokeWidth={1.75} aria-hidden="true" />
        <input
          className={styles.input}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(0);
          }}
          onKeyDown={onKeyDown}
          placeholder="Search pages and projects"
          aria-label="Search pages and projects"
          aria-controls="command-results"
          aria-activedescendant={results[active] ? `command-${results[active].id}` : undefined}
          role="combobox"
          aria-expanded="true"
          autoFocus
        />
        <kbd className={styles.kbd}>Esc</kbd>
      </div>
      <ul id="command-results" className={styles.list} role="listbox">
        {results.length === 0 ? <li className={styles.empty}>No pages or projects match “{query}”.</li> : null}
        {results.map((command, index) => {
          const Icon = command.icon;
          const first = index === 0 || results[index - 1].group !== command.group;
          return (
            <li key={command.id} role="presentation">
              {first ? <div className={styles.group}>{command.group}</div> : null}
              <div
                id={`command-${command.id}`}
                role="option"
                aria-selected={index === active}
                className={styles.option}
                onMouseMove={() => setActive(index)}
                onClick={() => choose(command)}
              >
                <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className={styles.label}>{command.label}</span>
                <span className={styles.hint}>{command.hint}</span>
              </div>
            </li>
          );
        })}
      </ul>
    </dialog>
  );
}
