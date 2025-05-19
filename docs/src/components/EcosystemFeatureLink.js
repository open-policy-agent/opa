import useBaseUrl from "@docusaurus/useBaseUrl";
import React from "react";

import entries from "@generated/ecosystem-data/default/entries.json";
import sortPagesByRank from "../lib/ecosystem/sortPagesByRank.js";

export default function EcosystemFeatureLink({ feature, children }) {
  const allPages = entries;

  const featurePages = [];

  for (const pageId in allPages) {
    const page = allPages[pageId];
    if (page.docs_features && page.docs_features[feature]) {
      featurePages.push(page);
    }
  }

  let message = "1 project";
  if (featurePages.length > 1) {
    message = `${featurePages.length} projects`;
  }

  return <a href={useBaseUrl(`/ecosystem/by-feature/${feature}`)}>{children} ({message})</a>;
}
