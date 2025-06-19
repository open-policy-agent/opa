import { React, useEffect } from "react";
import { useLocation } from "react-router-dom";

import { useThemeConfig } from "@docusaurus/theme-common";
import ReactMarkdown from "react-markdown";
import styles from "./styles.module.css";

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
      <div className={styles.commandHeader}>
        <h2 id={id} className={styles.commandTitle}>
          {id}
          <a href={`#${id}`} className={styles.directLink} title={`Direct link to ${id}`}>
            <span className={styles.directLinkSymbol}>#</span>
          </a>
        </h2>
      </div>

      {useline && (
        <pre className={styles.useline}>
          {useline}
        </pre>
      )}

      {long && <ReactMarkdown>{convertUrlsToMarkdownLinks(long)}</ReactMarkdown>}

      {flags.length > 0 && (
        <>
          <h3>Flags</h3>
          <table className={styles.flagsTable}>
            <thead>
              <tr>
                <th className={styles.flagsTableHeader}>Short</th>
                <th className={styles.flagsTableHeader}>Flag</th>
                <th className={styles.flagsTableHeader}>Description</th>
              </tr>
            </thead>
            <tbody>
              {flags.map((f, idx) => (
                <tr key={idx}>
                  <td className={styles.flagsTableCellWhitespace}>
                    {f.shorthand && <code>{f.shorthand}</code>}
                  </td>
                  <td className={styles.flagsTableCellWhitespace}>
                    <code>{f.name}</code>
                  </td>
                  <td className={styles.flagsTableCellWide}>
                    {f.description !== "" && (
                      <ReactMarkdown>
                        {convertUrlsToMarkdownLinks(capitalize(htmlSafe(f.description)))}
                      </ReactMarkdown>
                    )}

                    {f.type && f.type.startsWith("{") && (
                      <div className={styles.flagTypeContainer}>
                        Accepts:{" "}
                        <code className={styles.flagTypeCode}>
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
