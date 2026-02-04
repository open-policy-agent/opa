import React from "react";

import { useDoc } from "@docusaurus/plugin-content-docs/client";
import Content from "@theme-original/DocItem/Content";

import FeedbackForm from "@site/src/components/FeedbackForm";

export default function ContentWrapper(props) {
  const doc = useDoc();
  const showFeedbackForm = doc.frontMatter.show_feedback_form !== false;
  return (
    <>
      <Content {...props} />
      {showFeedbackForm && (
        <div className="feedback-form-wrapper">
          <FeedbackForm enablePopup={true} />
        </div>
      )}
    </>
  );
}
