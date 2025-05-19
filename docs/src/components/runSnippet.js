import BrowserOnly from "@docusaurus/BrowserOnly";
import React from "react";
import { useMemo } from "react";
import styled from "styled-components";

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

// hides the "by CodeAPI" text
const CodeAPIHider = styled.div`
  codapi-ref {
    display: none;
  }
`;

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
            <CodeAPIHider>
              <codapi-status done={`${icon} Done in $DURATION`}></codapi-status>
            </CodeAPIHider>
          </codapi-toolbar>
        </codapi-snippet>
      )}
    </BrowserOnly>
  );
}
