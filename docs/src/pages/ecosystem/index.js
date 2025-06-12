import React, { useState } from "react";

import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import Layout from "@theme/Layout";

import styles from "./styles.module.css";

import Card from "../../components/Card";
import CardGrid from "../../components/CardGrid";
import getLogoAsset from "../../lib/ecosystem/getLogoAsset.js";
import sortPagesByRank from "../../lib/ecosystem/sortPagesByRank.js";

import entries from "@generated/ecosystem-data/default/entries.json";
import featureCategories from "@generated/ecosystem-data/default/feature-categories.json";
import features from "@generated/ecosystem-data/default/features.json";
import languages from "@generated/ecosystem-data/default/languages.json";

const EcosystemIndex = (props) => {
  const title = "OPA Ecosystem";
  const sortedPages = sortPagesByRank(entries);

  const initialQuery = React.useMemo(() => {
    if (typeof window === "undefined") return "";
    return new URLSearchParams(window.location.search).get("q") || "";
  }, []);

  const [searchQuery, setSearchQuery] = useState(initialQuery);

  React.useEffect(() => {
    if (initialQuery) {
      window.location.hash = "#all-entries";
    }
  }, [initialQuery]);

  const filteredPages = sortedPages.filter((id) => {
    const page = entries[id];
    const query = searchQuery.toLowerCase();
    return (
      page.title.toLowerCase().includes(query)
      || page.subtitle?.toLowerCase().includes(query)
      || page.content.toLowerCase().includes(query)
    );
  });

  const orderedCategoryKeys = ["rego", "production", "tool", "createwithopa"];
  const preferredLanguageOrder = [
    "javascript",
    "java",
    "csharp",
    "golang",
    "swift",
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
        <Heading as="h1" className={styles.title}>
          {title}
        </Heading>
        <p className={styles.subtitle}>
          Showcase of OPA integrations, use-cases, and related projects.
        </p>

        <div className={styles.section}>
          <div className={styles.sectionContent}>
            <div className={styles.languageSection}>
              <Heading as="h2">Create With OPA</Heading>
              <p>Integrate with OPA from your language</p>
              <div className={styles.languageGrid}>
                {preferredLanguageOrder.map((langId) => {
                  const lang = languages[langId];
                  if (!lang) return null;

                  return (
                    <a
                      key={lang.id}
                      href={`./ecosystem/by-language/${lang.id}`}
                      className={styles.languageLink}
                    >
                      <img
                        src={require(`./assets/ecosystem/language-logos/${lang.id}.png`).default}
                        alt={`${lang.title} logo`}
                        className={styles.languageLogo}
                      />
                      <span className={styles.languageTitle}>{lang.title}</span>
                    </a>
                  );
                })}
              </div>
            </div>
          </div>
        </div>

        <div className={styles.section}>
          <div className={styles.sectionContent}>
            {allCategoryKeys.map((categoryId) => {
              const featuresInCategory = featureListByCategory[categoryId];
              if (!featuresInCategory) return null;

              const category = featureCategories[categoryId];
              const categoryTitle = category?.title;
              const categoryDescription = category?.description;

              return (
                <div key={categoryId} className={styles.categorySection}>
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

        <Heading as="h2" id="all-entries" className={styles.title}>
          All Entries & Integrations
        </Heading>

        <div className={styles.searchContainer}>
          <input
            type="text"
            placeholder="Search entries..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className={styles.searchInput}
          />
          <p className={styles.searchNote}>
            All integrations are ordered by the number of linked resources.{" "}
            <Link to="/docs/contrib-docs#opa-ecosystem-additions">Add yours!</Link>
          </p>
        </div>

        {filteredPages.length === 0
          ? (
            <p className={styles.noResults}>
              No integrations found. Try searching for something else or drop us a message on{" "}
              <a href="https://slack.openpolicyagent.org/" target="_blank" rel="noopener noreferrer">
                Slack
              </a>.
            </p>
          )
          : (
            <div className={styles.cardGridContainer}>
              <CardGrid>
                {filteredPages.map((id) => {
                  const page = entries[id];
                  const cardData = {
                    title: page.title,
                    note: page.subtitle,
                    icon: getLogoAsset(page.id),
                    link: `/ecosystem/entry/${id}`,
                    link_text: "View Details",
                  };

                  return <Card key={id} item={cardData} />;
                })}
              </CardGrid>
            </div>
          )}
      </div>
    </Layout>
  );
};

export default EcosystemIndex;
