import React, { useContext } from "react";

import InlineEditable from "../InlineEditable";
import ParamContext from "../ParamContext";

import styles from "./styles.module.css";

// Example Usage:
// <ParamProvider
//   initialParams={{
//     addr: 'localhost',
//     port: '8181',
//   }}
// >
//
// <ParamCodeBlock>
// {`opa run --server --addr {{addr}}:{{port}}`}
// </ParamCodeBlock>
//
// </ParamProvider>

// ParamCodeBlock is a component that displays a code block with inline editable
// parameters. This component only supports text language code blocks.
const ParamCodeBlock = ({ children }) => {
  const { params } = useContext(ParamContext);
  const code = children.trim();

  // Get the code with params substituted in for copying the complete command/text
  const getCodeToCopy = () => {
    return code.replace(/{{(\w+)}}/g, (_, key) => params[key] || "");
  };

  // split the code into text and param parts
  const parseCodeWithParams = (code) => {
    const regex = /{{(\w+)}}/g;
    let lastIndex = 0;
    const parts = [];
    let match;

    while ((match = regex.exec(code)) !== null) {
      if (match.index > lastIndex) {
        parts.push({ type: "text", content: code.slice(lastIndex, match.index) });
      }
      parts.push({ type: "param", content: match[1] });
      lastIndex = regex.lastIndex;
    }

    if (lastIndex < code.length) {
      parts.push({ type: "text", content: code.slice(lastIndex) });
    }

    return parts;
  };

  const parsedCode = parseCodeWithParams(code);

  return (
    <div className={styles.codeBlockContainer}>
      <pre className={styles.pre}>
        <div className={styles.codeBlock}>
          {parsedCode.map((part, index) => {
            if (part.type === "text") {
              return <span key={`text-${index}`}>{part.content}</span>;
            } else if (part.type === "param") {
              return <InlineEditable key={`param-${index}`} paramKey={part.content} />;
            }
            return null;
          })}
        </div>
      </pre>
    </div>
  );
};

export default ParamCodeBlock;
