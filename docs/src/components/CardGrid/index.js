import React from "react";

import styles from "./styles.module.css";

export default function CardGrid({ children, justifyCenter = true }) {
  return (
    <div className={`${styles.grid} ${justifyCenter ? styles.gridCenter : ''}`}>
      {children}
    </div>
  );
}
