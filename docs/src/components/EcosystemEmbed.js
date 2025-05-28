import useBaseUrl from "@docusaurus/useBaseUrl";
import React from "react";
import Card from "./Card";
import CardGrid from "./CardGrid";

import getLogoAsset from "../lib/ecosystem/getLogoAsset.js";
import sortPagesByRank from "../lib/ecosystem/sortPagesByRank.js";

import entries from "@generated/ecosystem-data/default/entries.json";

export default function EcosystemEmbed({ feature, children }) {
  const allPages = entries;

  const featurePages = [];

  for (const pageId in allPages) {
    const page = allPages[pageId];
    if (page.docs_features && page.docs_features[feature]) {
      featurePages.push(page);
    }
  }

  if (featurePages.length > 5) {
    return (
      <div style={{ marginTop: "2rem" }}>
        Browse {featurePages.length} projects related to "{feature}" in the{" "}
        <a href={useBaseUrl(`/ecosystem/by-feature/${feature}`)}>OPA Ecosystem</a>.
      </div>
    );
  }

  const sortedPages = sortPagesByRank(featurePages);

  return (
    <div style={{ marginTop: "2rem" }}>
      {children}
      <CardGrid>
        {sortedPages.map((id) => {
          const page = featurePages[id];
          if (!page) return null;

          const cardData = {
            title: page.title,
            note: page.docs_features[feature]?.note ?? "No note available",
            icon: getLogoAsset(page.id),
            link: useBaseUrl(`/ecosystem/entry/${page.id}`),
            link_text: "View Details",
          };

          return <Card key={id} item={cardData} />;
        })}
      </CardGrid>
    </div>
  );
}
