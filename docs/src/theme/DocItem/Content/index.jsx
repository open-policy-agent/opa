import React from "react";

import { useDoc } from "@docusaurus/plugin-content-docs/client";
import Content from "@theme-original/DocItem/Content";

import CopyPageMarkdown from "@site/src/components/CopyPageMarkdown";
import FeedbackForm from "@site/src/components/FeedbackForm";

export default function ContentWrapper(props) {
  const doc = useDoc();
  const showFeedbackForm = doc.frontMatter.show_feedback_form !== false;
  return (
    <>
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
