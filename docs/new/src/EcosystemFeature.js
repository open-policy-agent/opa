import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import ReactMarkdown from "react-markdown";
import Card from "./components/Card";

import sortPagesByRank from "./lib/sortPagesByRank";

import entries from "@generated/ecosystem-data/default/entries.json";
import featureCategories from "@generated/ecosystem-data/default/feature-categories.json";
import features from "@generated/ecosystem-data/default/features.json";
import languages from "@generated/ecosystem-data/default/languages.json";

const EcosystemFeature = (props) => {
  const { feature } = props.route.customData;

  const pagesByFeature = {};

  for (const pageId in entries) {
    const page = entries[pageId];
    const features = page.docs_features || {};

    for (const featureKey of Object.keys(features)) {
      if (!pagesByFeature[featureKey]) {
        pagesByFeature[featureKey] = [];
      }

      pagesByFeature[featureKey].push(page);
    }
  }

  const pages = pagesByFeature[feature] || [];

  const sortedPages = sortPagesByRank(pages);

  const title = features[feature].title;
  const content = features[feature].content;

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <Heading as="h1" style={{ margin: 0 }}>
          {title}
        </Heading>
        {content && (
          <div style={{ marginTop: "1rem" }}>
            <div style={{ marginTop: "0.5rem" }}>
              <ReactMarkdown>
                {content}
              </ReactMarkdown>
            </div>
          </div>
        )}

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
            const page = pages[id];

            const cardData = {
              title: page.title,
              note: page.docs_features[feature].note,
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
    </Layout>
  );
};

export default EcosystemFeature;
