import BrowserOnly from "@docusaurus/BrowserOnly";
import { PageMetadata } from "@docusaurus/theme-common";
import { translate } from "@docusaurus/Translate";
import Layout from "@theme/Layout";
import NotFoundContent from "@theme/NotFound/Content";
import React from "react";
export default function Index() {
  const title = translate({
    id: "theme.NotFound.title",
    message: "Page Not Found",
  });
  return (
    <>
      <PageMetadata title={title} />
      <Layout>
        <BrowserOnly>
          <NotFoundContent />
        </BrowserOnly>
      </Layout>
    </>
  );
}
