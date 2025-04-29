import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";

import versions from "@generated/versions-data/default/versions.json";

const Archive = (props) => {
  const title = "OPA Documentation Archive";
  const descVersions = versions.slice().reverse(); // Or just `versions` if already sorted descending

  const getArchiveUrl = (version) => {
    const urlVersionPart = version.replaceAll(".", "-");
    return `https://${urlVersionPart}--opa-docs.netlify.app/`;
  };

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <Heading as="h1">{title}</Heading>

        <p>Please find a list of archived OPA docs versions here:</p>

        <div style={{ marginTop: "1rem" }}>
          <ul
            style={{
              listStyle: "none",
              paddingLeft: 0,
            }}
          >
            {descVersions.map((version) => (
              <li key={version}>
                <a
                  href={getArchiveUrl(version)}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {version}
                </a>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Layout>
  );
};

export default Archive;
