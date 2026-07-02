import { Icon } from "@iconify/react";
import React from "react";
import styles from "./KapaSearchNavbarItem.module.css";

export default function KapaSearchNavbarItem() {
  const platform = typeof navigator !== "undefined"
    ? (navigator.userAgentData?.platform ?? navigator.platform)
    : "";
  const isMac = platform.includes("Mac");

  function openKapaSearch() {
    if (typeof window !== "undefined" && window.Kapa) {
      window.Kapa.open({ mode: "search" });
    }
  }

  return (
    <div className={styles.searchContainer}>
      <button onClick={openKapaSearch} className={styles.searchButton} aria-label="Search">
        <Icon icon="mdi:magnify" className={styles.searchIcon} aria-hidden="true" />
        <span>Search</span>
        <span className={styles.searchHints}>
          <kbd>{isMac ? "⌘" : "Ctrl"}</kbd>
          <kbd>K</kbd>
        </span>
      </button>
    </div>
  );
}
