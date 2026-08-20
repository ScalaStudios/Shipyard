import { Highlight, PrismTheme } from "prism-react-renderer";
import styles from "./CodeBlock.module.css";

const shipyardTheme: PrismTheme = {
  plain: { color: "var(--sy-color-text-primary)", backgroundColor: "transparent" },
  styles: [
    { types: ["comment", "prolog", "doctype", "cdata"], style: { color: "var(--sy-color-text-muted)", fontStyle: "italic" } },
    { types: ["punctuation"], style: { color: "var(--sy-color-text-muted)" } },
    { types: ["property", "tag", "constant", "symbol", "deleted"], style: { color: "var(--sy-color-status-info)" } },
    { types: ["boolean", "number"], style: { color: "var(--sy-color-status-warning)" } },
    { types: ["selector", "attr-name", "string", "char", "builtin", "inserted"], style: { color: "var(--sy-color-status-success)" } },
    { types: ["operator", "entity", "url", "variable"], style: { color: "var(--sy-color-text-primary)" } },
    { types: ["atrule", "attr-value", "function", "class-name"], style: { color: "var(--sy-color-action-primary-hover)" } },
    { types: ["keyword"], style: { color: "var(--sy-color-action-primary)" } },
    { types: ["regex", "important"], style: { color: "var(--sy-color-status-danger)" } },
  ],
};

export function CodeBlock({ code, language }: { code: string; language: string }) {
  return (
    <Highlight theme={shipyardTheme} code={code.trimEnd()} language={language}>
      {({ style, tokens, getLineProps, getTokenProps }) => (
        <pre className={styles.block} style={style}>
          {tokens.map((line, i) => (
            <span key={i} {...getLineProps({ line })} className={styles.line}>
              {line.map((token, k) => (
                <span key={k} {...getTokenProps({ token })} />
              ))}
            </span>
          ))}
        </pre>
      )}
    </Highlight>
  );
}
