import LatticeLoader from "../reactbits/LatticeLoader";

export function Loader({ label, status = "working", elapsed }: { label: string; status?: "working" | "done" | "error"; elapsed?: number }) {
  return (
    <LatticeLoader
      label={label}
      status={status}
      elapsed={elapsed}
      fontSize={13}
      color="var(--sy-color-action-primary)"
      doneColor="var(--sy-color-status-success)"
      errorColor="var(--sy-color-status-danger)"
    />
  );
}
