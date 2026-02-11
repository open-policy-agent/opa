import React from "react";
import SessionCard from "../SessionCard";
import styles from "./styles.module.css";

const AgendaItem = ({ item }) => {
  if (item.type === "session") {
    return <SessionCard {...item} />;
  }

  if (item.type === "booth") {
    return (
      <div className={styles.boothItem}>
        <div className={styles.boothDetails}>
          <div className={styles.boothDetailItem}>
            <strong>Booth:</strong> {item.location}
          </div>
          <div className={styles.boothDetailItem}>
            <strong>Hours:</strong> {item.hours}
          </div>
        </div>
      </div>
    );
  }

  return null;
};

export default AgendaItem;
