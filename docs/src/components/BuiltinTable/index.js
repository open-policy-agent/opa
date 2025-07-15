import React from "react";
import ReactMarkdown from "react-markdown";

import styles from "./styles.module.css";

import builtins from "@generated/builtin-data/default/builtins.json";

function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

export default function BuiltinTable({
  category,
  id,
  title,
  children,
}) {
  const categoryFns = builtins._categories[category];
  if (!categoryFns) return <p>No built-ins found for category "{category}".</p>;

  const htmlID = id || category;
  const htmlTitle = title || capitalize(category);

  // This component is used on a page that's so large it takes some time to
  // render. This means that something the anhor is not present on the page
  // and the browser is unable to find the element and go there itself.
  // Ideally, the built ins would have smaller pages but this is here to
  // preserve the functionality for now.
  React.useEffect(() => {
    const path = window.location.hash;

    // Exit early if there's no hash in the URL
    if (!path || !path.includes("#")) return;

    const id = path.replace("#", "");
    let attempts = 0;
    const maxAttempts = 15;
    const interval = 100;

    const tryScroll = () => {
      const el = document.getElementById(id);

      if (el) {
        const r = el.getBoundingClientRect();
        window.top.scroll({
          top: window.pageYOffset + r.top,
          behavior: "auto", // immediate jump
        });
      } else if (attempts < maxAttempts) {
        attempts += 1;
        setTimeout(tryScroll, interval);
      }
    };

    tryScroll();
  }, []);

  return (
    <div>
      <table style={{ width: "100%", tableLayout: "fixed" }}>
        <colgroup>
          <col />
          <col style={{ width: "100%" }} />
          <col />
        </colgroup>
        <thead>
          <tr>
            <th>Function</th>
            <th>Description</th>
            <th>Meta</th>
          </tr>
        </thead>
        <tbody>
          {categoryFns.map((name) => {
            const fn = builtins[name];
            if (!fn) return null;

            // we have links out there in the wild that use the name without a dot
            // this needs to be preserved for backwards compatibility.
            const anchor = `builtin-${category}-${name.replaceAll(".", "")}`;
            const isInfix = !!fn.infix;
            const isRelation = !!fn.relation;

            const args = fn.args || [];
            const result = fn.result || {};

            const signature = isInfix
              ? `${args[0]?.name || "x"} ${fn.infix} ${args[1]?.name || "y"}`
              : isRelation
              ? `${name}(${args.map((a) => a.name).join(", ")}, ${result.name})`
              : `${result.name || "result"} := ${name}(${args.map((a) => a.name).join(", ")})`;

            return (
              <tr key={anchor} id={anchor}>
                <td className={styles.functionCell}>
                  <a href={`#${anchor}`}>
                    <code className={styles.functionName}>
                      {(isInfix ? signature : name)
                        .split(/(\.|_)/)
                        .map((part, index) =>
                          part === "." || part === "_"
                            ? (
                              <React.Fragment key={index}>
                                {part}
                                <wbr />
                              </React.Fragment>
                            )
                            : part
                        )}
                    </code>
                  </a>
                </td>
                <td>
                  <p>
                    <code>{signature}</code>
                  </p>
                  {fn.description && <ReactMarkdown>{fn.description}</ReactMarkdown>}

                  {args.length > 0 && (
                    <div>
                      <strong>Arguments:</strong>
                      {args.map((arg, i) => (
                        <div key={i} style={{ marginBottom: "0.5rem" }}>
                          <div>
                            <code>{arg.name}</code> <span>({arg.type})</span>
                          </div>
                          <div>
                            <ReactMarkdown>{arg.description}</ReactMarkdown>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  <strong>Returns:</strong>
                  <div>
                    <code>{result.name}</code> <span>({result.type})</span>
                    <div>
                      <ReactMarkdown>{result.description}</ReactMarkdown>
                    </div>
                  </div>
                </td>
                <td>
                  <div>
                    {fn.introduced && fn.introduced !== "edge" && fn.introduced !== "v0.17.0" && (
                      <a
                        href={`https://github.com/open-policy-agent/opa/releases/${fn.introduced}`}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <span>{fn.introduced}</span>
                      </a>
                    )}
                    {fn.introduced === "edge" && <span>edge</span>} {fn.wasm
                      ? (
                        <span
                          style={{
                            backgroundColor: "seagreen",
                            color: "white",
                            fontWeight: "bold",
                            fontSize: "0.8rem",
                            padding: "0.1rem 0.2rem",
                            borderRadius: "0.2rem",
                          }}
                        >
                          Wasm
                        </span>
                      )
                      : (
                        <span
                          style={{
                            backgroundColor: "darkgoldenrod",
                            color: "white",
                            fontWeight: "bold",
                            fontSize: "0.8rem",
                            padding: "0.1rem 0.2rem",
                            borderRadius: "0.2rem",
                            whiteSpace: "nowrap",
                          }}
                        >
                          SDK-dependent
                        </span>
                      )}
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {children}
    </div>
  );
}
