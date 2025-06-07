import React from "react";

import ReactMarkdown from "react-markdown";

import Link from "@docusaurus/Link";

import styles from "./styles.module.css";

export default function Card({ item }) {
  return (
    <div className={styles.card}>
      <div className={styles.header}>
        {item.icon && <img src={item.icon} alt="" className={styles.icon} />}
        <h3 className={styles.title}>{item.title}</h3>
      </div>
      {item.note && (
        <div className={styles.note}>
          <ReactMarkdown>{item.note}</ReactMarkdown>
        </div>
      )}
      {item.link && (
        <Link to={item.link} className={styles.link}>
          {item.link_text || "Learn more"}
        </Link>
      )}
    </div>
  );
}
