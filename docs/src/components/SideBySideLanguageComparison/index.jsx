import { MDXProvider } from "@mdx-js/react";
import CodeBlock from "@theme/CodeBlock";
import * as components from "@theme/MDXComponents"; // Import default MDX components from Docusaurus theme
import React from "react";
import ReactMarkdown from "react-markdown";

import SideBySideColumn from "../SideBySide/Column";
import SideBySideContainer from "../SideBySide/Container";

import styles from "./styles.module.css";

export default function SideBySideLanguageComparison({
  title,
  intro,
  outro,
  title1,
  title2,
  code1,
  code2,
  lang1,
  lang2,
}) {
  return (
    <div>
      <h2>{title}</h2>
      <ReactMarkdown>{intro}</ReactMarkdown>
      <SideBySideContainer>
        <SideBySideColumn>
          <h3>{title1}</h3>
          <MDXProvider components={components}>
            <CodeBlock language={lang1}>
              {code1}
            </CodeBlock>
          </MDXProvider>
        </SideBySideColumn>
        <SideBySideColumn>
          <h3>{title2}</h3>
          <MDXProvider components={components}>
            <CodeBlock language={lang2}>
              {code2}
            </CodeBlock>
          </MDXProvider>
        </SideBySideColumn>
      </SideBySideContainer>
      <div className={styles.outro}>
        <ReactMarkdown>{outro}</ReactMarkdown>
      </div>
    </div>
  );
}
