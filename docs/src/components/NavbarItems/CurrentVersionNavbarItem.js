// Based on implementation outlined here:
// https://github.com/facebook/docusaurus/issues/7227#issue-1212117180
//
import React from "react";

import BrowserOnly from "@docusaurus/BrowserOnly";
import useBaseUrl from "@docusaurus/useBaseUrl";
import semver from "semver";

import DefaultNavbarItem from "@theme/NavbarItem/DefaultNavbarItem";

import styles from "./styles.module.css";

// display: inline-block overrides the default and ensures the item shows on
// mobile at the top of the page.
export default function CurrentVersionNavbarItem({ ...props }) {
  const baseUrl = useBaseUrl("/");
  const normalizedBaseUrl = baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;
  const href = useBaseUrl("/docs/archive");

  // on the mobile menu, show nothing. Note, the 'Desktop' item has 'display'
  // set, so it'll appear on mobile too.
  if (props.mobile === true) return null;

  return (
    <BrowserOnly fallback={null}>
      {() => {
        const path = window.location.pathname;
        if (!path.startsWith(`${normalizedBaseUrl}docs`)) {
          return null;
        }

        return (
          <div className={styles.versionWrapper}>
            <DefaultNavbarItem
              {...props}
              label={"edge"}
              href={href}
            />
          </div>
        );
      }}
    </BrowserOnly>
  );
}
