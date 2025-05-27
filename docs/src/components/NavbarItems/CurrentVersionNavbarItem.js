// Based on implementation outlined here:
// https://github.com/facebook/docusaurus/issues/7227#issue-1212117180
//
import BrowserOnly from "@docusaurus/BrowserOnly";
import useBaseUrl from "@docusaurus/useBaseUrl";
import versions from "@generated/versions-data/default/versions.json";
import DefaultNavbarItem from "@theme/NavbarItem/DefaultNavbarItem";
import React from "react";
import semver from "semver";
import styled from "styled-components";

// display: inline-block overrides the default and ensures the item shows on
// mobile at the top of the page.
const VersionWrapper = styled.div`
  a.navbar__item.navbar__link {
    display: inline-block;
  }
`;

export default function CurrentVersionNavbarItem({ ...props }) {
  const baseUrl = useBaseUrl("/");
  const normalizedBaseUrl = baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;
  const href = useBaseUrl("/docs/archive");
  const latestVersion = versions.filter(semver.valid).sort(semver.rcompare)[0];

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
          <VersionWrapper>
            <DefaultNavbarItem
              {...props}
              label={`${latestVersion}`}
              href={href}
            />
          </VersionWrapper>
        );
      }}
    </BrowserOnly>
  );
}
