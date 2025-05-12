import Link from "@docusaurus/Link";
import { MDXProvider } from "@mdx-js/react";
import CodeBlock from "@theme/CodeBlock";
import * as components from "@theme/MDXComponents"; // Import default MDX components from Docusaurus theme
import React from "react";

import RunSnippet from "./runSnippet";
import SideBySideColumn from "./SideBySide/Column";
import SideBySideContainer from "./SideBySide/Container";

export default function PlaygroundExample({
  dir,
}) {
  let files = dir.keys().reduce((acc, key) => {
    let fileName = key.replace(`./`, "");
    if (!fileName.includes(".")) {
      return acc;
    }
    if (!fileName.endsWith(".json")) {
      acc[fileName] = dir(key).default;
    } else {
      acc[fileName] = dir(key);
    }
    return acc;
  }, {});

  const config = files["config.json"];

  const input = files["input.json"];
  const data = files["data.json"];
  const policy = files["policy.rego"];

  const title = files["title.txt"];
  const intro = files["intro.md"];
  const outro = files["outro.md"];

  const output = files["output.json"];

  const showInput = config?.showInput ?? true;
  const showData = config?.showData ?? true;
  const showTitles = config?.showTitles ?? true;
  const command = config?.command ?? "data.play";

  const state = encodeToBase64(JSON.stringify({
    i: JSON.stringify(input, null, 2),
    d: JSON.stringify(data, null, 2),
    p: policy,
  }));

  const url = `https://play.openpolicyagent.org/?state=${state}`;

  const showNotes = output && output.some(rule => rule.note);

  let dataString = JSON.stringify(data, null, 2);
  if (config && config.showData && config.dataLineLimit) {
    dataString = dataString.split("\n").slice(0, config.dataLineLimit).join("\n") + "\n...";
  }

  // id is used to stop contents from other examples on the same page being used
  const id = getId(state);

  const snippetFiles = `#${id}-input.json:input.json #${id}-data.json:data.json`;

  return (
    <div>
      {title && <h2>{title}</h2>}

      {intro && intro()}

      {showInput && (
        <SideBySideContainer>
          <SideBySideColumn>
            <MDXProvider components={components}>
              <CodeBlock language={"rego"} title="policy.rego">
                {policy}
              </CodeBlock>
              <RunSnippet command="data.play" id={`${id}-policy.rego`} files={snippetFiles} />
            </MDXProvider>
          </SideBySideColumn>
          <SideBySideColumn>
            <MDXProvider components={components}>
              <CodeBlock language={"json"} title="input.json">
                {JSON.stringify(input, null, 2)}
              </CodeBlock>
              <RunSnippet id={`${id}-input.json`} />
            </MDXProvider>

            {showData && (
              <MDXProvider components={components}>
                <CodeBlock language={"json"} title="data.json">
                  {dataString}
                </CodeBlock>
                <RunSnippet id={`${id}-data.json`} />
              </MDXProvider>
            )}
          </SideBySideColumn>
        </SideBySideContainer>
      )}

      {!showData && (
        <div className="dn">
          {/* this is needed to include the contents of data.json, but hidden when the config turned it off */}
          <CodeBlock language={"json"} title="data.json">
            {JSON.stringify(data, null, 2)}
          </CodeBlock>
          <RunSnippet id={`${id}-data.json`} />
        </div>
      )}

      {!showInput && (
        <MDXProvider components={components}>
          {/* this is needed to include the contents of input.json, but hidden when the config turned it off */}
          <div className="dn">
            <CodeBlock language={"json"} title="input.json">
              {JSON.stringify(input, null, 2)}
            </CodeBlock>
            <RunSnippet id={`${id}-input.json`} />
          </div>
          <CodeBlock language={"rego"} title={showTitles ? "policy.rego" : ""}>
            {policy}
          </CodeBlock>
          <RunSnippet command={command} id={`${id}-policy.rego`} files={snippetFiles} playgroundLink={url} />
        </MDXProvider>
      )}

      {output && (
        <p>
          <Link to={url}>Open in OPA Playground</Link>
        </p>
      )}

      {output && (
        <table>
          <thead>
            <tr>
              <th>Rule</th>
              <th>Output Value</th>
              {showNotes && <th>Notes</th>}
            </tr>
          </thead>
          <tbody>
            {output.map((rule, index) => {
              return (
                <tr key={index}>
                  <td>{rule.ref}</td>
                  <td>
                    {rule.value !== undefined
                      && rule.value !== "undefined"
                      && <code>{JSON.stringify(rule.value)}</code>}
                    {rule.value === "undefined" && <code>undefined</code>}
                  </td>
                  {showNotes && <td>{rule.note}</td>}
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {outro && outro()}
    </div>
  );
}

function encodeToBase64(str) {
  const utf8Bytes = new TextEncoder().encode(str);
  const base64String = btoa(String.fromCharCode.apply(null, utf8Bytes));
  return base64String;
}

// djb2 http://www.cse.yorku.ca/~oz/hash.html
function getId(str) {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = (hash * 33) ^ str.charCodeAt(i);
  }
  return (hash >>> 0).toString(36).slice(0, 6);
}
