import StatusMark, { type StatusMarkStatus } from "../reactbits/StatusMark";

function markStatus(status: string): StatusMarkStatus {
  switch (status) {
    case "succeeded":
    case "completed":
    case "success":
      return "done";
    case "failed":
    case "error":
      return "failed";
    case "running":
    case "leased":
    case "leasing":
      return "running";
    case "canceled":
    case "cancelled":
    case "skipped":
    case "skipped_exists":
      return "cancelled";
    default:
      return "pending";
  }
}

export function RunStatus({ status, label }: { status: string; label?: string }) {
  return (
    <StatusMark
      status={markStatus(status)}
      label={label ?? status}
      size={16}
      fontSize={13}
      strike={false}
      color="var(--sy-color-status-info)"
      doneColor="var(--sy-color-status-success)"
      errorColor="var(--sy-color-status-danger)"
    />
  );
}
