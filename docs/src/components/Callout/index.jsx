import React from "react";

import styles from "./styles.module.css";

export default function Callout({ children }) {
  return <div className={styles.callout}>{children}</div>;
}
