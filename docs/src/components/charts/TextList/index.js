import React from 'react';
import styles from './styles.module.css';

export default function TextList({ intro, bullets, config }) {
  return (
    <div className={styles.container}>
      {intro && (
        <p className={styles.intro}>{intro}</p>
      )}
      {bullets && bullets.length > 0 && (
        <ul className={styles.bullets}>
          {bullets.map((bullet, index) => (
            <li key={index} className={styles.bulletItem}>
              <strong>{bullet.title}</strong> ({bullet.count} {bullet.count === 1 ? 'response' : 'responses'}): {bullet.sentence}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
