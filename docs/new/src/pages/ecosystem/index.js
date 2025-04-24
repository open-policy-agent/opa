import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";

import Card from "../../components/Card";
import getLogoAsset from "../../lib/ecosystem/getLogoAsset.js";
import sortPagesByRank from "../../lib/ecosystem/sortPagesByRank.js";

import entries from "@generated/ecosystem-data/default/entries.json";
import featureCategories from "@generated/ecosystem-data/default/feature-categories.json";
import features from "@generated/ecosystem-data/default/features.json";
import languages from "@generated/ecosystem-data/default/languages.json";

const EcosystemIndex = (props) => {
  const sortedPages = sortPagesByRank(entries);
  const title = "OPA Ecosystem";

  const orderedCategoryKeys = ["rego", "production", "tool", "createwithopa"];
  const preferredLanguageOrder = [
    "javascript",
    "java",
    "csharp",
    "golang",
    "clojure",
    "rust",
    "php",
  ];

  const featureListByCategory = {};

  Object.values(features).forEach((feature) => {
    const category = feature.category || "uncategorized";
    if (!featureListByCategory[category]) {
      featureListByCategory[category] = [];
    }
    featureListByCategory[category].push(feature);
  });

  const allCategoryKeys = [
    ...orderedCategoryKeys,
    ...Object.keys(featureListByCategory).filter((key) => !orderedCategoryKeys.includes(key)),
  ];

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <Heading as="h1" style={{ margin: 0 }}>
          {title}
        </Heading>
        <p style={{ fontSize: "1.2rem", color: "#555" }}>
          Showcase of OPA integrations, use-cases, and related projects.
        </p>

        <div style={{ marginTop: "3rem" }}>
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: "2rem",
              justifyContent: "space-between",
            }}
          >
            <div style={{ flex: "1 1 calc(50% - 1rem)", minWidth: "300px" }}>
              <Heading as="h2">Create With OPA</Heading>
              <p>Integrate with OPA from your language</p>
              <div style={{ display: "flex", flexWrap: "wrap", gap: "2rem", marginTop: "1.5rem" }}>
                {preferredLanguageOrder.map((langId) => {
                  const lang = languages[langId];
                  if (!lang) return null;

                  return (
                    <a
                      key={lang.id}
                      href={`./ecosystem/by-language/${lang.id}`}
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "center",
                        width: "100px",
                        textAlign: "center",
                        textDecoration: "none",
                        color: "inherit",
                      }}
                    >
                      <img
                        src={require(`./assets/ecosystem/language-logos/${lang.id}.png`).default}
                        alt={`${lang.title} logo`}
                        style={{
                          width: "64px",
                          height: "64px",
                          objectFit: "contain",
                          marginBottom: "0.5rem",
                        }}
                      />
                      <span style={{ fontSize: "0.9rem", fontWeight: 500 }}>{lang.title}</span>
                    </a>
                  );
                })}
              </div>
            </div>
          </div>
        </div>

        <div style={{ marginTop: "3rem" }}>
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: "2rem",
              justifyContent: "space-between",
            }}
          >
            {allCategoryKeys.map((categoryId) => {
              const featuresInCategory = featureListByCategory[categoryId];
              if (!featuresInCategory) return null;

              const category = featureCategories[categoryId];
              const categoryTitle = category?.content.match(/title: \|\n\s+(.*)/)?.[1] || categoryId;
              const categoryDescription = category?.content.match(/description: \|\n((?:\s+.+\n?)+)/)?.[1]
                ?.split("\n")
                .map((line) => line.trim())
                .join(" ") || "";

              return (
                <div key={categoryId} style={{ flex: "1 1 calc(50% - 1rem)", minWidth: "300px" }}>
                  <Heading as="h2">{categoryTitle}</Heading>
                  <p>{categoryDescription}</p>
                  <ul>
                    {featuresInCategory.map((feature) => (
                      <li key={feature.id}>
                        <a href={`./ecosystem/by-feature/${feature.id}`}>{feature.title}</a> – {feature.description}
                      </li>
                    ))}
                  </ul>
                </div>
              );
            })}
          </div>
        </div>

        <Heading as="h2" style={{ margin: 0 }}>
          All Entries & Integrations
        </Heading>

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
            const page = entries[id];

            const cardData = {
              title: page.title,
              note: page.subtitle,
              icon: getLogoAsset(page.id),
              link: `/ecosystem/entry/${id}`,
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

export default EcosystemIndex;
