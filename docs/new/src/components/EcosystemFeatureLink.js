import React from "react";

import entries from "@generated/ecosystem-data/default/entries.json";

export default function EcosystemFeatureLink({ feature, children }) {
  const allPages = entries;

  const featurePages = [];

  for (const pageId in allPages) {
    const page = allPages[pageId];
    if (page.docs_features && page.docs_features[feature]) {
      featurePages.push(page);
    }
  }

  const sortedPages = sortPagesByRank(featurePages);

  return <span>{feature}</span>;
}
