import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button, Panel, StatusBadge } from "@shipyard/ui";
import table from "../components/DataTable.module.css";
import { PageHeader } from "../components/PageHeader";
import { useWorkspace } from "../context/WorkspaceContext";
import { api, OnboardingStatus } from "../api";
import styles from "./OnboardingPage.module.css";

export function OnboardingPage() {
  const { setError } = useWorkspace();
  const [status, setStatus] = useState<OnboardingStatus | null>(null);

  useEffect(() => {
    void api
      .onboarding()
      .then(setStatus)
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load onboarding"));
  }, []);

  const steps = status?.steps ?? [];
  const done = steps.filter((s) => s.done).length;

  return (
    <div className={table.stack}>
      <PageHeader
        title="Get started"
        description="Everything needed to take this instance from empty to shipping. Steps update as you complete them."
      />
      <Panel
        title="Setup checklist"
        meta={
          <StatusBadge status={status?.complete ? "success" : "info"}>
            {done}/{steps.length}
          </StatusBadge>
        }
      >
        <ol className={styles.steps}>
          {steps.map((step) => (
            <li key={step.id} className={step.done ? styles.stepDone : styles.step}>
              <span className={step.done ? styles.markDone : styles.mark} aria-hidden="true">
                {step.done ? "✓" : ""}
              </span>
              <div className={styles.body}>
                <div className={styles.title}>
                  {step.title}
                  {step.optional ? <span className={styles.optional}>optional</span> : null}
                </div>
                <p className={styles.detail}>{step.detail}</p>
              </div>
              <div className={styles.action}>
                {step.done ? (
                  <StatusBadge status="success">done</StatusBadge>
                ) : (
                  <Link to={step.href}>
                    <Button type="button" variant="secondary">
                      {step.action}
                    </Button>
                  </Link>
                )}
              </div>
            </li>
          ))}
        </ol>
      </Panel>
    </div>
  );
}
