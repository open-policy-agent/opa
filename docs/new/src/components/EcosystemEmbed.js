import React from "react";
import ReactMarkdown from "react-markdown";
import Card from "./Card";

import sortPagesByRank from "../lib/sortPagesByRank";

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

  const sortedPages = sortPagesByRank(featurePages);

  return (
    <div className="margin-vert--lg">
      {children}
      <div
        style={{
          marginTop: "2rem",
          display: "flex",
          flexWrap: "wrap",
          justifyContent: "center",
          gap: 20,
        }}
      >
        {sortedPages.map((id) => {
          const page = featurePages[id];
          if (!page) return null;

          const cardData = {
            title: page.title,
            note: page.docs_features[feature]?.note ?? "No note available",
            icon: page.logo,
            link: `/ecosystem/entry/${page.id}`,
            link_text: "View Details",
          };

          return (
            <div key={id} style={{ flex: "1 1 30%", minWidth: "250px", maxWidth: "400px" }}>
              <Card item={cardData} />
            </div>
          );
        })}
      </div>
    </div>
  );
}
