import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import BrowserOnly from "@docusaurus/BrowserOnly";
import styles from "./styles.module.css";

export default function CopyPageMarkdown() {
  return (
    <BrowserOnly>
      {() => <CopyButtonPortal />}
    </BrowserOnly>
  );
}

function CopyButtonPortal() {
  const [container, setContainer] = useState(null);

  useLayoutEffect(() => {
    // Find the page heading and insert a container after it
    const heading = document.querySelector(
      "article .markdown > header, article .markdown > h1, article .markdown > h2",
    );
    if (!heading) return;
    const el = document.createElement("div");
    heading.insertAdjacentElement("afterend", el);
    setContainer(el);
    return () => el.remove();
  }, []);

  if (!container) return null;
  return createPortal(<CopyButton />, container);
}

function CopyButton() {
  const [copied, setCopied] = useState(false);
  const copyTimeout = useRef(undefined);

  useEffect(() => () => window.clearTimeout(copyTimeout.current), []);

  const handleClick = useCallback(async () => {
    const article = document.querySelector("article");
    if (!article) return;

    const markdownDiv = article.querySelector(".markdown");
    if (!markdownDiv) return;

    const clone = markdownDiv.cloneNode(true);

    // Strip UI-only elements before conversion
    const selectorsToRemove = [
      "[data-copy-exclude]", // components that opt out of copy
      ".hash-link", // Docusaurus heading anchor icons
      ".buttonGroup", // Docusaurus code block copy/wrap buttons
      "[hidden]", // hidden elements
    ];
    for (const sel of selectorsToRemove) {
      for (const el of clone.querySelectorAll(sel)) {
        el.remove();
      }
    }
    const TurndownService = (await import("turndown")).default;
    const { gfm } = await import("turndown-plugin-gfm");

    const turndown = new TurndownService({
      headingStyle: "atx",
      codeBlockStyle: "fenced",
    });

    turndown.use(gfm);

    // Always render tables as GFM, compressing multi-line cell content
    turndown.addRule("docusaurusTable", {
      filter(node) {
        return node.nodeName === "TABLE";
      },
      replacement(_content, node) {
        const rows = node.rows;
        if (!rows || rows.length === 0) return _content;

        const cellText = (cell) =>
          turndown.turndown(cell.innerHTML)
            .replace(/\n/g, " ")
            .replace(/\|/g, "\\|")
            .trim();

        const headerCells = Array.from(rows[0].cells).map(cellText);
        const lines = [];
        lines.push("| " + headerCells.join(" | ") + " |");
        lines.push("| " + headerCells.map(() => "---").join(" | ") + " |");

        for (let i = 1; i < rows.length; i++) {
          const cells = Array.from(rows[i].cells).map(cellText);
          lines.push("| " + cells.join(" | ") + " |");
        }

        return "\n\n" + lines.join("\n") + "\n\n";
      },
    });

    let markdown = turndown.turndown(clone.innerHTML);

    // Post-process: clean up markdown artifacts
    markdown = markdown
      // Strip deep heading markers (####+) that leak into table cells;
      // also removes h4+ headings elsewhere, which is acceptable
      .replace(/#{4,}\s*/g, "")
      // Collapse runs of 3+ blank lines to 2
      .replace(/\n{3,}/g, "\n\n")
      .trim();

    try {
      await navigator.clipboard.writeText(markdown);
    } catch {
      // Fallback for 'insecure' contexts (e.g. localhost over HTTP)
      const textarea = document.createElement("textarea");
      textarea.value = markdown;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    setCopied(true);
    window.clearTimeout(copyTimeout.current);
    copyTimeout.current = window.setTimeout(() => setCopied(false), 2000);
  }, []);

  const label = copied ? "Copied!" : "Copy Content for Chatbot or LLM";

  return (
    <div className={styles.wrapper} data-copy-exclude>
      <button
        className={`button button--secondary button--sm ${copied ? "button--success" : ""}`}
        onClick={handleClick}
        title={label}
        aria-label={label}
      >
        {label}
      </button>
    </div>
  );
}
