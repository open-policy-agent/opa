import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import ReactMarkdown from "react-markdown";

import Card from "./components/Card";
import CardGrid from "./components/CardGrid";

import getLogoAsset from "./lib/ecosystem/getLogoAsset.js";
import sortPagesByRank from "./lib/ecosystem/sortPagesByRank.js";

import entries from "@generated/ecosystem-data/default/entries.json";
import featureCategories from "@generated/ecosystem-data/default/feature-categories.json";
import features from "@generated/ecosystem-data/default/features.json";
import languages from "@generated/ecosystem-data/default/languages.json";

const EcosystemFeature = (props) => {
  const { language } = props.route.customData;

  const pagesByLanguage = {};

  for (const pageId in entries) {
    const page = entries[pageId];
    const lang = page.for_language;
    if (!lang) continue;
    if (!pagesByLanguage[lang]) {
      pagesByLanguage[lang] = [];
    }
    pagesByLanguage[lang].push(page);
  }

  const pages = pagesByLanguage[language] || [];

  const sortedPages = sortPagesByRank(pages);

  const title = languages[language].title;
  const content = languages[language].content;

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

        <CardGrid>
          {sortedPages.map((id) => {
            const page = pages[id];

            const cardData = {
              title: page.title,
              note: page.subtitle,
              icon: getLogoAsset(page.id),
              link: `/ecosystem/entry/${page.id}`,
              link_text: "View Details",
            };

            return <Card item={cardData} />;
          })}
        </CardGrid>
      </div>
    </Layout>
  );
};

export default EcosystemFeature;
