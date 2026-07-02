import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import BrowserOnly from "@docusaurus/BrowserOnly";
import IconCopy from "@theme/Icon/Copy";
import IconSuccess from "@theme/Icon/Success";

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

    const TurndownService = (await import("turndown")).default;
    const { gfm } = await import("turndown-plugin-gfm");
    const { createMarkdownConverter, COPY_EXCLUDE_SELECTORS } = await import("@site/src/lib/pageMarkdown.js");

    for (const sel of COPY_EXCLUDE_SELECTORS) {
      for (const el of clone.querySelectorAll(sel)) {
        el.remove();
      }
    }

    const convert = createMarkdownConverter(TurndownService, gfm);
    const markdown = convert(clone.innerHTML);

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
        <span className={styles.icons} aria-hidden="true">
          {copied ? <IconSuccess /> : <IconCopy />}
        </span>
        {label}
      </button>
    </div>
  );
}
