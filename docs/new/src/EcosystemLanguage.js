import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import ReactMarkdown from "react-markdown";
import Card from "./components/Card";

import sortPagesByRank from "./lib/sortPagesByRank";

const EcosystemFeature = (props) => {
  const { pages, language, languages } = props.route.customData;

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
              note: page.subtitle,
              icon: page.logo,
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

export default EcosystemFeature;
