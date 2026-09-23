import styles from "./IconLinkNavbarItem.module.css";

export default function IconLinkNavbarItem({ href, label, path }) {
  return (
    <div className="navbar__item">
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={label}
        className={styles.iconButton}
      >
        <svg viewBox="0 0 24 24" className={styles.icon} aria-hidden="true">
          <path fill="currentColor" d={path} />
        </svg>
      </a>
    </div>
  );
}
