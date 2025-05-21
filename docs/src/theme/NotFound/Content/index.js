import Translate from "@docusaurus/Translate";
import Heading from "@theme/Heading";
import clsx from "clsx";
import React from "react";

export default function NotFoundContent({ className }) {
  const currentPath = window.location.pathname;
  const currentHost = window.location.host;

  const fullUrl = `https://${currentHost}${currentPath}`;

  const issueTitle = `docs: 404 error on ${currentPath}`;
  const issueBody = `## Description

The page at path **${currentPath}** resulted in a 404 error.

### URL:
[${fullUrl}](${fullUrl})

### Additional Context:

I found the broken link on https://...

<!-- If you encountered an issue, please help us fix it by providing more details. -->
`;

  const encodedTitle = encodeURIComponent(issueTitle);
  const encodedBody = encodeURIComponent(issueBody);
  const labels = "docs,bug";

  const githubIssueUrl =
    `https://github.com/open-policy-agent/opa/issues/new?template=bug_report.md&title=${encodedTitle}&body=${encodedBody}&labels=${labels}`;

  return (
    <main className={clsx("container margin-vert--xl", className)}>
      <div className="row">
        <div className="col col--6 col--offset-3">
          <Heading as="h1" className="hero__title">
            <Translate
              id="theme.NotFound.title"
              description="The title of the 404 page"
            >
              Sorry, looks like this page was lost at sea!
            </Translate>
          </Heading>
          <p>
            <Translate
              id="theme.NotFound.p1"
              description="The first paragraph of the 404 page"
            >
              We could not find what you were looking for.
            </Translate>
          </p>
          <p>
            <Translate
              id="theme.NotFound.p2"
              description="The 2nd paragraph of the 404 page"
            >
              This is a bug, please help us fix it!
            </Translate>
          </p>
          <p>
            <Translate
              id="theme.NotFound.p3"
              description="The 3rd paragraph of the 404 page"
            >
              When creating the issue, please make sure to reference the following path:
            </Translate>
          </p>
          <pre className="language-text">
            {currentPath}
          </pre>

          <div className="text--center">
            <a
              href={githubIssueUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="button button--primary button--sm"
            >
              Report a Bug
            </a>
          </div>
        </div>
      </div>
    </main>
  );
}
