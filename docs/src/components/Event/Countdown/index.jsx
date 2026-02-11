import Heading from "@theme/Heading";
import React, { useEffect, useState } from "react";

import styles from "./styles.module.css";

const Countdown = ({ targetDate, title }) => {
  const [timeRemaining, setTimeRemaining] = useState(null);

  useEffect(() => {
    const MS_PER_SECOND = 1000;
    const MS_PER_MINUTE = MS_PER_SECOND * 60;
    const MS_PER_HOUR = MS_PER_MINUTE * 60;
    const MS_PER_DAY = MS_PER_HOUR * 24;

    const updateCountdown = () => {
      const now = new Date();
      const diff = targetDate - now;

      if (diff <= 0) {
        setTimeRemaining(null);
        return;
      }

      const days = Math.floor(diff / MS_PER_DAY);
      const hours = Math.floor((diff % MS_PER_DAY) / MS_PER_HOUR);
      const minutes = Math.floor((diff % MS_PER_HOUR) / MS_PER_MINUTE);
      const seconds = Math.floor((diff % MS_PER_MINUTE) / MS_PER_SECOND);

      setTimeRemaining({ days, hours, minutes, seconds });
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, MS_PER_SECOND);
    return () => clearInterval(interval);
  }, [targetDate]);

  if (!timeRemaining) {
    return null;
  }

  return (
    <>
      <Heading as="h1" className={styles.countdownTitle}>
        {title}
      </Heading>
      <div className={styles.countdown}>
        <div className={styles.countdownItem}>
          <div className={styles.countdownNumber}>{timeRemaining.days}</div>
          <div className={styles.countdownLabel}>Days</div>
        </div>
        <div className={styles.countdownItem}>
          <div className={styles.countdownNumber}>{timeRemaining.hours}</div>
          <div className={styles.countdownLabel}>Hours</div>
        </div>
        <div className={styles.countdownItem}>
          <div className={styles.countdownNumber}>{timeRemaining.minutes}</div>
          <div className={styles.countdownLabel}>Minutes</div>
        </div>
        <div className={styles.countdownItem}>
          <div className={styles.countdownNumber}>{timeRemaining.seconds}</div>
          <div className={styles.countdownLabel}>Seconds</div>
        </div>
      </div>
    </>
  );
};

export default Countdown;
