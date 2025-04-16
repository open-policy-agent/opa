import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";
import Card from "./components/Card";

const EcosystemIndex = (props) => {
  const pages = props.route.customData.content.pages;

  const sortedPages = sortPagesByRank(pages);

  const title = "OPA Ecosystem";

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <Heading as="h1" style={{ margin: 0 }}>
          {title}
        </Heading>

        <div style={{ marginTop: "2rem", display: "flex", flexWrap: "wrap", gap: 20 }}>
          {sortedPages.map((id) => {
            const page = pages[id];

            const cardData = {
              title: page.title,
              note: page.subtitle,
              icon: page.logo,
              link: `/ecosystem/${id}`,
              link_text: "View Details",
            };

            return (
              <div key={id} style={{ flex: "1 1 30%", minWidth: "250px" }}>
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

function sortPagesByRank(pages) {
  // Create a copy of pages to avoid mutating the input
  const integrationsRanked = [];

  // Iterate through each page and calculate its rank based on tutorials, videos, blogs, and code
  Object.keys(pages).forEach((id) => {
    const page = pages[id];
    const params = page;

    let tutorialsCount = 0;
    if (params.tutorials && Array.isArray(params.tutorials)) {
      const uniqueHosts = [...new Set(params.tutorials.map(url => new URL(url).host))];
      tutorialsCount = uniqueHosts.length;
    }

    let videosCount = 0;
    if (params.videos && Array.isArray(params.videos)) {
      videosCount = params.videos.length;
    }

    let blogsCount = 0;
    if (params.blogs && Array.isArray(params.blogs)) {
      const uniqueHosts = [...new Set(params.blogs.map(url => new URL(url).host))];
      blogsCount = uniqueHosts.length;
    }

    let codeCount = 0;
    if (params.code && Array.isArray(params.code)) {
      codeCount = params.code.length;
    }

    // Calculate the rank by summing all the counts
    const rank = tutorialsCount + videosCount + blogsCount + codeCount;

    // Add the page and its rank to the integrationsRanked array
    integrationsRanked.push({
      id,
      rank,
    });
  });

  // Sort the pages by rank in descending order
  integrationsRanked.sort((a, b) => b.rank - a.rank);

  // Return the sorted list of page IDs based on rank
  return integrationsRanked.map((integration) => integration.id);
}
