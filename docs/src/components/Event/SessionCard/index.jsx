import Link from "@docusaurus/Link";
import { Icon } from "@iconify/react";
import React from "react";

import styles from "./styles.module.css";

const SessionCard = ({ sessionType, title, datetime, room, location, speakers, link }) => {
  const speakersText = speakers && speakers.length > 0
    ? speakers.map(s => `${s.name} (${s.title || s.affiliation || ""})`).join(", ")
    : null;

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <h4 className={styles.title}>
          {sessionType === "lightning" && <Icon icon="lucide:zap" className={styles.lightningIcon} />}
          {title}
        </h4>
      </div>

      <div className={styles.details}>
        <div className={styles.detailItem}>
          <strong>When:</strong> {datetime}
        </div>
        {room && (
          <div className={styles.detailItem}>
            <strong>Room:</strong> {room}
          </div>
        )}
        {location && (
          <div className={styles.detailItem}>
            <strong>Location:</strong> {location}
          </div>
        )}
        {speakersText && (
          <div className={styles.detailItem}>
            <strong>Speaker{speakers.length > 1 ? "s" : ""}:</strong> {speakersText}
          </div>
        )}
        {link && (
          <div className={styles.detailItem}>
            <Link to={link} className={styles.inlineLink}>
              View on Schedule →
            </Link>
          </div>
        )}
      </div>
    </div>
  );
};

export default SessionCard;
