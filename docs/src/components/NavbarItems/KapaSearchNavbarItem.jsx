import { Icon } from "@iconify/react";
import styles from "./KapaSearchNavbarItem.module.css";

export default function KapaSearchNavbarItem() {
  function openKapaChat() {
    if (typeof window !== "undefined" && window.Kapa) {
      window.Kapa.open();
    }
  }

  return (
    <div className={styles.item}>
      <button onClick={openKapaChat} className={styles.chatButton} aria-label="Ask AI">
        <Icon icon="mdi:robot-outline" className={styles.chatIcon} aria-hidden="true" />
        <span>Ask AI</span>
      </button>
    </div>
  );
}
