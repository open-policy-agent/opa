import React from "react";

import styles from "./styles.module.css";

export default function SideBySideContainer({ children }) {
  return <div className={styles.container}>{children}</div>;
}
