import React from "react";

import Admonition from "@theme/Admonition";
import Heading from "@theme/Heading";
import Layout from "@theme/Layout";

const semver = require("semver");

import versions from "@generated/versions-data/default/versions.json";

const Archive = (props) => {
  const title = "OPA Documentation Archive";

  const lastOldDocsVersion = "v1.4.2";
  const oldDocsVersions = [
    "v0.11.0",
    "v0.12.2",
    "v0.13.5",
    "v0.14.2",
    "v0.15.1",
    "v0.16.2",
    "v0.17.3",
    "v0.18.0",
    "v0.19.2",
    "v0.20.5",
    "v0.21.1",
    "v0.22.0",
    "v0.23.2",
    "v0.24.0",
    "v0.25.2",
    "v0.26.0",
    "v0.27.1",
    "v0.28.0",
    "v0.29.4",
    "v0.30.2",
    "v0.31.0",
    "v0.32.1",
    "v0.33.1",
    "v0.34.2",
    "v0.35.0",
    "v0.36.1",
    "v0.37.2",
    "v0.38.1",
    "v0.39.0",
    "v0.40.0",
    "v0.41.0",
    "v0.42.2",
    "v0.43.1",
    "v0.44.0",
    "v0.45.0",
    "v0.46.3",
    "v0.47.4",
    "v0.48.0",
    "v0.49.2",
    "v0.50.2",
    "v0.51.0",
    "v0.52.0",
    "v0.53.1",
    "v0.54.0",
    "v0.55.0",
    "v0.56.0",
    "v0.57.1",
    "v0.58.0",
    "v0.59.0",
    "v0.60.0",
    "v0.61.0",
    "v0.62.1",
    "v0.63.0",
    "v0.64.1",
    "v0.65.0",
    "v0.66.0",
    "v0.67.1",
    "v0.68.0",
    "v0.69.0",
    "v0.70.0",
    "v1.0.1",
    "v1.1.0",
    "v1.2.0",
    "v1.3.0",
    "v1.4.2",
  ];

  const firstDocsVersion = semver.valid("0.17.2");

  // We only show the latest patch for each minor release
  const getLatestPatchVersions = (versions) => {
    const versionGroups = {};

    versions.forEach(version => {
      const parsed = semver.parse(version);
      if (!parsed) return;

      const majorMinorKey = `${parsed.major}.${parsed.minor}`;

      if (!versionGroups[majorMinorKey] || semver.gt(version, versionGroups[majorMinorKey])) {
        versionGroups[majorMinorKey] = version;
      }
    });

    return Object.values(versionGroups);
  };

  // A known list of old docs versions are shown, otherwise, any newer version
  // is shown.
  const filteredVersions = versions.slice().filter(version => {
    return semver.gt(version, lastOldDocsVersion) || oldDocsVersions.includes(version);
  });

  // For versions > lastOldDocsVersion, group by minor and get only latest patch
  // For versions in oldDocsVersions, keep as is
  const newerVersions = filteredVersions.filter(v => semver.gt(v, lastOldDocsVersion));
  const olderVersions = filteredVersions.filter(v => oldDocsVersions.includes(v));

  const latestNewerVersions = getLatestPatchVersions(newerVersions);

  // Combine all selected versions, and sort them newest to oldest
  const descVersions = [...latestNewerVersions, ...olderVersions].sort(semver.rcompare);

  const getArchiveUrl = (version) => {
    const urlVersionPart = version.replaceAll(".", "-");
    return `https://${urlVersionPart}--opa-docs.netlify.app/`;
  };

  return (
    <Layout title={title}>
      <div className="container margin-vert--lg">
        <Heading as="h1">{title}</Heading>

        <Admonition type="info" title="Note">
          Pre-release documentation is available on the{" "}
          <a href="https://edge--opa-docs.netlify.app/">edge deployment</a>.
        </Admonition>

        <p>
          Please find links to past versions of the OPA Documentation here. Note that only the latest patch within a
          minor release is shown.
        </p>

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
