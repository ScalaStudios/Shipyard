import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@shipyard/ui/styles.css";
import { App } from "./app/App";
import "./styles/global.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
