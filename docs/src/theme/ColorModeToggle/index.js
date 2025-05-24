// This file has been ejected from Docusaurus core in order to support a
// customization relating to re-highlighting.
import { translate } from "@docusaurus/Translate";
import useIsBrowser from "@docusaurus/useIsBrowser";
import IconDarkMode from "@theme/Icon/DarkMode";
import IconLightMode from "@theme/Icon/LightMode";
import clsx from "clsx";
import React from "react";
import styles from "./styles.module.css";
function ColorModeToggle({ className, buttonClassName, value, onChange }) {
  const isBrowser = useIsBrowser();
  const title = translate(
    {
      message: "Switch between dark and light mode (currently {mode})",
      id: "theme.colorToggle.ariaLabel",
      description: "The ARIA label for the navbar color mode toggle",
    },
    {
      mode: value === "dark"
        ? translate({
          message: "dark mode",
          id: "theme.colorToggle.ariaLabel.mode.dark",
          description: "The name for the dark color mode",
        })
        : translate({
          message: "light mode",
          id: "theme.colorToggle.ariaLabel.mode.light",
          description: "The name for the light color mode",
        }),
    },
  );
  return (
    <div className={clsx(styles.toggle, className)}>
      <button
        className={clsx(
          "clean-btn",
          styles.toggleButton,
          !isBrowser && styles.toggleButtonDisabled,
          buttonClassName,
        )}
        type="button"
        onClick={() => {
          onChange(value === "dark" ? "light" : "dark");

          // TODO: file an issue and check if this issue has been fixed this is
          // a workaround for an issue when on manually toggling the color
          // mode, the highlighting is not re-run
          if (typeof window !== "undefined") {
            window.location.reload();
          }
        }}
        disabled={!isBrowser}
        title={title}
        aria-label={title}
        aria-live="polite"
        aria-pressed={value === "dark" ? "true" : "false"}
      >
        <IconLightMode
          className={clsx(styles.toggleIcon, styles.lightToggleIcon)}
        />
        <IconDarkMode
          className={clsx(styles.toggleIcon, styles.darkToggleIcon)}
        />
      </button>
    </div>
  );
}
export default React.memo(ColorModeToggle);
