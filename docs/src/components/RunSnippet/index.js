import { React, useMemo } from "react";

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

export default function RunSnippet({ id, files, depends, command, playgroundLink }) {
  if (!command && !files) {
    // json file
    return (
      <BrowserOnly>
        {() => <codapi-snippet editor="basic" id={id}></codapi-snippet>}
      </BrowserOnly>
    );
  }

  const icon = useMemo(() => emojis[Math.floor(Math.random() * emojis.length)], []);

  return (
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
          init-delay="500" // we need this for codapi-toolbar to work
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
  );
}
