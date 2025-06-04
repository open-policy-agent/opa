import React, { useEffect, useState } from "react";

import BrowserOnly from "@docusaurus/BrowserOnly";
import { Icon } from "@iconify/react";
import Admonition from "@theme/Admonition";

import styles from "./styles.module.css";

export default function FeedbackForm() {
  const formName = "page-feedback";
  const [feedbackType, setFeedbackType] = useState("");
  const [comment, setComment] = useState("");
  const [url, setUrl] = useState("");
  const [path, setPath] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (typeof window !== "undefined") {
      if (window.location.hostname === "localhost") {
        // setMessage("Feedback form disabled on localhost.");
        // return;
      }
      setUrl(window.location);
      setPath(window.location.pathname);
    }
  }, []);

  useEffect(() => {
    const feedbackSubmitted = localStorage.getItem(`${formName}-${path}`);
    if (feedbackSubmitted) {
      setMessage("Thank you for your feedback!");
    }
  }, [path]);

  const handleSubmit = (event) => {
    event.preventDefault();

    const formData = new FormData(event.target);

    fetch("/", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams(formData).toString(),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP Status: ${response.status}`);
        }

        localStorage.setItem(`${formName}-${path}`, "submitted");
        setMessage("Thank you for your feedback!");
      })
      .catch((error) => {
        console.error(error);
        setMessage("Oh dear, there was an error submitting feedback...");
      });
  };

  if (message !== "") {
    return (
      <BrowserOnly>
        {() => {
          return (
            <Admonition
              className={styles.feedbackFormWrapper}
              type="note"
              icon={<Icon icon="mdi:feedback-outline" size="48px" />}
              title="Feedback"
            >
              <p>{message}</p>

              <p>
                Got more to say? Questions for the OPA experts? Please come and find us on the{" "}
                <strong>
                  <a href="https://slack.openpolicyagent.org">OPA Slack</a>.
                </strong>{" "}
                The <code>#help</code> channel is a great place to get stated.
              </p>
            </Admonition>
          );
        }}
      </BrowserOnly>
    );
  }

  // NOTE: remember to update the static/netlify-forms.html file so that they
  // know which files to permit. If this is not updated, new fields in the
  // form will be dropped.
  return (
    <BrowserOnly>
      {() => {
        return (
          <Admonition
            className={styles.feedbackFormWrapper}
            type="note"
            icon={<Icon icon="mdi:feedback-outline" size="48px" />}
            title="Feedback"
          >
            <p>We are always trying to make our documentation the best it can be and welcome your comments.</p>
            <form
              className={styles.feedbackForm}
              name={formName}
              method="post"
              onSubmit={handleSubmit}
            >
              <input type="hidden" name="form-name" value={formName} />
              <input type="hidden" name="url" value={url} />
              <input type="hidden" name="feedback-type" value={feedbackType} />

              <div className={styles.section} style={{ display: "flex", gap: "10px" }}>
                <button
                  type="button"
                  onClick={() => setFeedbackType("positive")}
                  className={`${styles.feedbackFormButton} ${feedbackType === "positive" ? styles.positive : ""}`}
                >
                  <Icon icon="mdi:thumbs-up" size="48px" />
                </button>
                <button
                  type="button"
                  onClick={() => setFeedbackType("negative")}
                  className={`${styles.feedbackFormButton} negative ${
                    feedbackType === "negative" ? styles.negative : ""
                  }`}
                >
                  <Icon icon="mdi:thumbs-down" size="48px" />
                </button>
              </div>

              {feedbackType && (
                <div className={styles.section}>
                  <div className={styles.comment}>
                    <textarea
                      name="comment"
                      rows="4"
                      placeholder={feedbackType === "positive"
                        ? "What do you like about this page? (optional)"
                        : "What could be improved on this page?"}
                      value={comment}
                      onChange={(e) => setComment(e.target.value)}
                      required={feedbackType === "negative"}
                    />
                  </div>
                  <div className={styles.emailRole}>
                    <div>
                      <input type="email" name="email" placeholder="Email (optional)" />
                    </div>
                  </div>
                  <p className={styles.emailNote}>Email will only be used for comment follow-up.</p>
                </div>
              )}

              {feedbackType && (
                <div className={styles.section}>
                  <button className={styles.feedbackFormButton} type="submit">
                    <Icon icon="mdi:send" size="48px" />
                  </button>
                </div>
              )}
            </form>
          </Admonition>
        );
      }}
    </BrowserOnly>
  );
}
