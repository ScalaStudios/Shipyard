import { FormEvent, useEffect, useState } from "react";
import { Button } from "@shipyard/ui";
import styles from "./LoginPage.module.css";
import { api, OIDCProvider } from "./api";

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
  const [providers, setProviders] = useState<OIDCProvider[]>([]);

  useEffect(() => {
    void api
      .oidcProviders()
      .then((res) => setProviders(res.providers ?? []))
      .catch(() => setProviders([]));
  }, []);

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

  function providerLabel(p: OIDCProvider) {
    switch (p.kind) {
      case "github":
        return "Continue with GitHub";
      case "gitlab":
        return "Continue with GitLab";
      case "forgejo":
        return "Continue with Forgejo";
      case "gitea":
        return "Continue with Gitea";
      default:
        return `Continue with ${p.name}`;
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.atmosphere} aria-hidden="true" />
      <div className={styles.stage}>
        <aside className={styles.hero}>
          <img className={styles.heroMark} src="/shipyard-mark.svg" width={72} height={72} alt="" />
          <div className={styles.heroBrand}>Shipyard</div>
          <p className={styles.heroLine}>
            Source → pipeline → artifact → release → deployment.
            <br />
            One control plane for delivery.
          </p>
        </aside>

        <form className={styles.panel} onSubmit={onSubmit}>
          <div className={styles.panelHead}>
            <img src="/shipyard-mark.svg" width={28} height={28} alt="" />
            <div>
              <div className={styles.brand}>Shipyard</div>
              <h1>{mode === "login" ? "Sign in" : "Create first account"}</h1>
            </div>
          </div>
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

          {error ? (
            <div className={styles.error} role="alert">
              {error}
            </div>
          ) : null}

          <Button type="submit" variant="primary" loading={loading}>
            {mode === "login" ? "Sign in" : "Create account"}
          </Button>

          {providers.length > 0 ? (
            <div className={styles.oauth}>
              <div className={styles.oauthDivider}>or</div>
              {providers.map((p) => (
                <a key={p.name} className={styles.oauthLink} href={`/api/v1/auth/oidc/${p.name}/start`}>
                  {providerLabel(p)}
                </a>
              ))}
            </div>
          ) : null}

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
    </div>
  );
}
