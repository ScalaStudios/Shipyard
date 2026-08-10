import { FormEvent, useState } from "react";
import { Button } from "@shipyard/ui";
import styles from "./LoginPage.module.css";
import { api } from "./api";

export function LoginPage({
  allowRegister,
  onAuthed,
}: {
  allowRegister: boolean;
  onAuthed: () => void;
}) {
  const [mode, setMode] = useState<"login" | "register">(allowRegister ? "register" : "login");
  const [login, setLogin] = useState("");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      if (mode === "register") {
        await api.register({ username, email, password });
      } else {
        await api.login({ login, password });
      }
      onAuthed();
    } catch (err) {
      setError(err instanceof Error ? err.message : "authentication failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className={styles.page}>
      <form className={styles.panel} onSubmit={onSubmit}>
        <div className={styles.brand}>Shipyard</div>
        <h1>{mode === "login" ? "Sign in" : "Create first account"}</h1>
        <p className={styles.copy}>
          {mode === "login"
            ? "Access the delivery control plane with your local account."
            : "Bootstrap the first operator account for this Shipyard instance."}
        </p>

        {mode === "register" ? (
          <>
            <label className={styles.field}>
              <span>Username</span>
              <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
            </label>
            <label className={styles.field}>
              <span>Email</span>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" required />
            </label>
          </>
        ) : (
          <label className={styles.field}>
            <span>Username or email</span>
            <input value={login} onChange={(e) => setLogin(e.target.value)} autoComplete="username" required />
          </label>
        )}

        <label className={styles.field}>
          <span>Password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === "login" ? "current-password" : "new-password"}
            required
            minLength={8}
          />
        </label>

        {error ? <div className={styles.error} role="alert">{error}</div> : null}

        <Button type="submit" variant="primary" loading={loading}>
          {mode === "login" ? "Sign in" : "Create account"}
        </Button>

        {allowRegister ? (
          <button
            type="button"
            className={styles.switch}
            onClick={() => setMode((m) => (m === "login" ? "register" : "login"))}
          >
            {mode === "login" ? "Need an account? Register" : "Already registered? Sign in"}
          </button>
        ) : null}
      </form>
    </div>
  );
}
