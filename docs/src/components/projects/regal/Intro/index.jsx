import React from "react";

import Link from "@docusaurus/Link";

import styles from "./styles.module.css";

export default function Intro({ image }) {
  return (
    <div className={styles.container}>
      <div className={styles.column}>
        <img className={styles.logo} src={image} />
      </div>
      <div className={styles.column}>
        <blockquote>
          regal<br />
          adj : of notable excellence or magnificence : splendid
          <br />
          -- <a href="https://www.merriam-webster.com/dictionary/regal">Merriam-Webster</a>
        </blockquote>
      </div>
    </div>
  );
}
