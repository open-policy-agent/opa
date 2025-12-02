import Translate from "@docusaurus/Translate";
import Heading from "@theme/Heading";
import clsx from "clsx";
import React from "react";

export default function NotFoundContent({ className }) {
  const currentPath = window.location.pathname;
  const currentHost = window.location.host;

  const fullUrl = `https://${currentHost}${currentPath}`;

  const encodedBrokenLink = encodeURIComponent(fullUrl);

  const githubIssueUrl =
    `https://github.com/open-policy-agent/opa/issues/new?template=broken-link.yaml&broken-link=${encodedBrokenLink}`;

  return (
    <main className={clsx("container margin-vert--xl", className)}>
      <div className="row">
        <div className="col col--6 col--offset-3">
          <Heading as="h1" className="hero__title">
            Sorry, this link is broken.
          </Heading>
          <p>
            <strong>Will you help us fix it?</strong>{" "}
            All you need to do is open an issue with some information about where you just clicked this link.
          </p>
          <Heading as="h3">
            Step 1: Get the URL you just came from
          </Heading>
          <div>
            <p>
              This is the most important step. We don't track you and so we don't know which site you just came from.
            </p>
            <p>
              Press back in your browser and copy the URL of the page you just visited. We'll need this in the next
              step. Press forward once you have it copied.
            </p>
          </div>
          <Heading as="h3">
            Step 2: Click this button to open the issue template
          </Heading>
          <div>
            <p>
              This link will pre-populate the broken link information. You'll just need to fill in where you found this link.
            </p>
            <p>
              <a
                href={githubIssueUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="button button--primary"
              >
                Report a Bug
              </a>
            </p>
            <p>
              This will open in a new tab.
            </p>
          </div>
        </div>
      </div>
    </main>
  );
}
