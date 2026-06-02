import React from "react";

import Head from "@docusaurus/Head";
import { useDoc } from "@docusaurus/plugin-content-docs/client";
import Content from "@theme-original/DocItem/Content";

import CopyPageMarkdown from "@site/src/components/CopyPageMarkdown";
import FeedbackForm from "@site/src/components/FeedbackForm";

export default function ContentWrapper(props) {
  const doc = useDoc();
  const showFeedbackForm = doc.frontMatter.show_feedback_form !== false;
  const mdHref = `${doc.metadata.permalink}.md`;
  return (
    <>
      <Head>
        <link rel="alternate" type="text/markdown" href={mdHref} />
      </Head>
      <Content {...props} />
      <CopyPageMarkdown />
      {showFeedbackForm && (
        <div className="feedback-form-wrapper" data-copy-exclude>
          <FeedbackForm enablePopup={true} />
        </div>
      )}
    </>
  );
}
