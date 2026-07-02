import { React, useEffect, useMemo, useRef, useState } from "react";

import BrowserOnly from "@docusaurus/BrowserOnly";

import styles from "./styles.module.css";

const emojis = [
  "💖",
  "😀",
  "😃",
  "🚕",
  "🏎",
  "🛸",
  "🎉",
  "📬",
  "🔩",
  "❤️‍🔥",
  "✅",
  "🆗",
  "🗯",
  "🏁",
  "💯",
  "💕",
  "🩵",
  "🧿",
  "🎈",
  "🎁",
  "🤖",
  "🥳",
];

export default function RunSnippet({ id, files, depends, command, playgroundLink, output }) {
  const [isLoading, setIsLoading] = useState(true);
  const [showInitialOutput, setShowInitialOutput] = useState(!!output);
  const snippetRef = useRef(null);
  const loadDelay = 500;

  useEffect(() => {
    const timer = setTimeout(() => {
      setIsLoading(false);
    }, loadDelay);

    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!output || !snippetRef.current) return;
    const el = snippetRef.current.querySelector("codapi-snippet");
    if (!el) return;
    const handler = () => setShowInitialOutput(false);
    el.addEventListener("result", handler);
    return () => el.removeEventListener("result", handler);
  }, [output, isLoading]);

  if (!command && !files) {
    // json file
    return (
      <BrowserOnly>
        {() => <codapi-snippet editor="basic" id={id} data-copy-exclude></codapi-snippet>}
      </BrowserOnly>
    );
  }

  const icon = useMemo(() => emojis[Math.floor(Math.random() * emojis.length)], []);

  return (
    <>
      <div ref={snippetRef}>
        <BrowserOnly>
          {() => (
            <codapi-snippet
              sandbox="javascript"
              engine="playground"
              editor="basic"
              id={id}
              command={command}
              files={files}
              depends-on={depends}
              init-delay={500} // we need this for codapi-toolbar to work
              className={isLoading ? styles.dn : styles.codeApiSnippet}
              data-copy-exclude
            >
              <codapi-toolbar>
                <button>Evaluate</button>
                <a href="#edit">Edit</a>
                {playgroundLink && <a target="_blank" href={playgroundLink}>Open in Playground</a>}
                <div className={styles.codeApiHider}>
                  <codapi-status done={`${icon} Done in $DURATION`}></codapi-status>
                </div>
              </codapi-toolbar>
            </codapi-snippet>
          )}
        </BrowserOnly>
      </div>
      {/* Output is hidden visually but included in the DOM for copy-as-markdown */}
      {showInitialOutput && (
        <div>
          <div className={styles.dn}>Output</div>
          <pre>{output}</pre>
        </div>
      )}
      {/* must be at the end or it'll become the codapi policy */}
      <div className={isLoading ? styles.codeApiSnippetLoadingPlaceholder : styles.dn} data-copy-exclude>
        Loading...
      </div>
    </>
  );
}
