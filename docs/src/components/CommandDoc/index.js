import { useThemeConfig } from "@docusaurus/theme-common";
import React from "react";
import { useEffect } from "react";
import ReactMarkdown from "react-markdown";
import { useLocation } from "react-router-dom";

const capitalize = (str) => {
  return str.charAt(0).toUpperCase() + str.slice(1);
};

const convertUrlsToMarkdownLinks = (text) => {
  if (typeof text !== "string") {
    text = String(text);
  }

  text = text.replace(/<\/?[^>]+(>|$)/g, "");

  const urlRegex = /https?:\/\/(?:www\.)?[^\s<>"'()]+[^\s<>"'.),!?]/g;

  return text.replace(urlRegex, (rawUrl) => {
    const trailingMatch = rawUrl.match(/[.)]+$/);
    const trailing = trailingMatch ? trailingMatch[0] : "";
    const url = trailing ? rawUrl.slice(0, -trailing.length) : rawUrl;

    const displayText = url
      .replace(/^https?:\/\//, "")
      .replace(/^www\./, "")
      .replace(/\/$/, "");

    return `[${displayText}](${url})${trailing}`;
  });
};

const htmlSafe = (str) => {
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
};

const CommandDoc = ({ command }) => {
  const { navbar } = useThemeConfig();
  const location = useLocation();

  const {
    id,
    use,
    useline,
    long,
    example,
    flags,
  } = command;

  return (
    <div>
      <div style={{ marginBottom: "0.5rem" }}>
        <h2 id={id} style={{ display: "inline" }}>
          {id}
          <a href={`#${id}`} style={{ marginLeft: "0.3rem" }} title={`Direct link to ${id}`}>
            <span style={{ color: "var(--ifm-link-color)" }}>#</span>
          </a>
        </h2>
      </div>

      {useline && (
        <pre style={{ padding: "0.5em", borderRadius: "var(--ifm-code-border-radius)" }}>
          {useline}
        </pre>
      )}

      {long && <ReactMarkdown>{convertUrlsToMarkdownLinks(long)}</ReactMarkdown>}

      {flags.length > 0 && (
        <>
          <h3>Flags</h3>
          <table style={{ fontSize: "0.9em", borderCollapse: "collapse", width: "100%" }}>
            <thead>
              <tr>
                <th style={{ textAlign: "left", padding: "0.5em" }}>Short</th>
                <th style={{ textAlign: "left", padding: "0.5em" }}>Flag</th>
                <th style={{ textAlign: "left", padding: "0.5em" }}>Description</th>
              </tr>
            </thead>
            <tbody>
              {flags.map((f, idx) => (
                <tr key={idx}>
                  <td style={{ whiteSpace: "nowrap", padding: "0.5em" }}>
                    {f.shorthand && <code>{f.shorthand}</code>}
                  </td>
                  <td style={{ whiteSpace: "nowrap", padding: "0.5em" }}>
                    <code>{f.name}</code>
                  </td>
                  <td style={{ padding: "0.5em", width: "100%" }}>
                    {f.description !== "" && (
                      <ReactMarkdown>
                        {convertUrlsToMarkdownLinks(capitalize(htmlSafe(f.description)))}
                      </ReactMarkdown>
                    )}

                    {f.type && f.type.startsWith("{") && (
                      <div style={{ marginTop: "0.25em" }}>
                        Accepts:{" "}
                        <code style={{ wordBreak: "break-word" }}>
                          {f.type.replace(/,/g, ",\u200b")}
                        </code>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {example && example.trim() !== "" && (
        <>
          <h3>Example</h3>
          <ReactMarkdown>{example}</ReactMarkdown>
        </>
      )}
    </div>
  );
};

export default CommandDoc;
