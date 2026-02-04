import { Icon } from "@iconify/react";
import React, { useEffect, useRef, useState } from "react";

import BrowserOnly from "@docusaurus/BrowserOnly";
import Admonition from "@theme/Admonition";

import styles from "./styles.module.css";

/**
 * A feedback form component that is automatically added to the end of documentation pages.
 * It can also be manually included in other pages by setting enablePopup to true.
 * The form allows users to provide positive or negative feedback with optional comments
. Feedback is submitted to Netlify Forms via AJAX.
 *
 * Note: When adding new fields to this form, remember to update the static/netlify-forms.html
 * file so that Netlify knows which fields to permit. If this is not updated, new fields
 * in the form will be dropped.
 */
export default function FeedbackForm({ enablePopup = false }) {
  // this is used to ensure the response is categorized in netlify forms and that
  // local state is remembered.
  const formName = "page-feedback";

  const [feedbackType, setFeedbackType] = useState("");
  const [comment, setComment] = useState("");
  const [url, setUrl] = useState("");
  const [path, setPath] = useState("");
  const [message, setMessage] = useState("");

  const [showFloatingPopup, setShowFloatingPopup] = useState(false);
  const [hasScrolled, setHasScrolled] = useState(false);
  const [popupEnabled, setPopupEnabled] = useState(enablePopup);
  const feedbackFormRef = useRef(null);

  // The popup will never show before the user has been on the page for 10s
  const feedbackPopupTimeoutMs = 10000;

  useEffect(() => {
    if (typeof window !== "undefined") {
      setUrl(window.location);
      setPath(window.location.pathname);

      // Check if popup has been globally dismissed
      const popupDismissed = localStorage.getItem("documentation-feedback-popup-dismissed");
      if (popupDismissed) {
        setPopupEnabled(false);
      }
    }
  }, []);

  // This effect sets up an intersection observer to detect when the feedback form
  // comes into view. When the form is visible, it permanently disables the popup
  // to prevent it from showing again. This ensures users who have found the form
  // don't get interrupted by the popup.
  useEffect(() => {
    if (feedbackFormRef.current) {
      const observer = new IntersectionObserver(
        (entries) => {
          if (entries[0].isIntersecting) {
            setShowFloatingPopup(false);
            setPopupEnabled(false);
          }
        },
        { threshold: 0.1 },
      );

      observer.observe(feedbackFormRef.current);

      return () => {
        observer.disconnect();
      };
    }
  }, [feedbackFormRef.current]);

  useEffect(() => {
    if (popupEnabled) {
      const handleScroll = () => {
        if (window.scrollY > 100) {
          setHasScrolled(true);
          window.removeEventListener("scroll", handleScroll);
        }
      };

      window.addEventListener("scroll", handleScroll);

      const timer = setTimeout(() => {
        if (hasScrolled) {
          setShowFloatingPopup(true);
        }
      }, feedbackPopupTimeoutMs);

      return () => {
        clearTimeout(timer);
        window.removeEventListener("scroll", handleScroll);
      };
    }
  }, [popupEnabled, hasScrolled]);

  // feedback is stored in local storage to prevent the form / popup from showing again
  useEffect(() => {
    const feedbackSubmitted = localStorage.getItem(`${formName}-${path}`);
    if (feedbackSubmitted) {
      setMessage("Thank you for your feedback!");
      setPopupEnabled(false);
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
        setShowFloatingPopup(false);
      })
      .catch((error) => {
        console.error(error);
        setMessage("Oh dear, there was an error submitting feedback...");
      });
  };

  const scrollToFeedback = () => {
    if (feedbackFormRef.current) {
      feedbackFormRef.current.scrollIntoView({ behavior: "smooth" });
      setShowFloatingPopup(false);
    }
  };

  if (message !== "") {
    return (
      <BrowserOnly>
        {() => (
          <Admonition
            className={styles.wrapper}
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
        )}
      </BrowserOnly>
    );
  }

  return (
    <BrowserOnly>
      {() => (
        <>
          {popupEnabled && showFloatingPopup && (
            <FloatingPopup
              onClose={() => setShowFloatingPopup(false)}
              onFeedbackSelect={(type) => {
                setFeedbackType(type);
                scrollToFeedback();
              }}
            />
          )}
          <div ref={feedbackFormRef}>
            <Admonition
              className={styles.wrapper}
              type="note"
              icon={<Icon icon="mdi:feedback-outline" size="48px" />}
              title="Feedback"
            >
              <p>We are always trying to make our documentation the best it can be and welcome your comments.</p>
              <Form
                formName={formName}
                url={url}
                feedbackType={feedbackType}
                setFeedbackType={setFeedbackType}
                comment={comment}
                setComment={setComment}
                handleSubmit={handleSubmit}
              />
            </Admonition>
          </div>
        </>
      )}
    </BrowserOnly>
  );
}

/**
 * A floating popup that appears after scrolling to encourage users to provide feedback.
 * The popup shows thumbs up/down buttons that, when clicked, will scroll the user to
 * the feedback form and pre-select their feedback type. The popup is automatically
 * hidden when the feedback form comes into view.
 */
const FloatingPopup = ({ onClose, onFeedbackSelect }) => {
  const [showAnimation, setShowAnimation] = useState(false);

  useEffect(() => {
    // Trigger fade-in animation after component mounts
    const timer = setTimeout(() => {
      setShowAnimation(true);
    }, 50); // Small delay to ensure smooth animation

    return () => clearTimeout(timer);
  }, []);

  const handleClose = () => {
    // Set global flag to never show popup again
    localStorage.setItem("documentation-feedback-popup-dismissed", "true");
    onClose();
  };

  const handleFeedbackSelect = (type) => {
    if (type === "negative") {
      localStorage.setItem("documentation-feedback-popup-dismissed", "true");
    }
    onFeedbackSelect(type);
  };

  return (
    <div className={`${styles.popup} ${showAnimation ? styles.show : ""}`}>
      <button
        className={styles.closeButton}
        onClick={handleClose}
        aria-label="Close feedback popup"
      >
        <Icon icon="mdi:close" size="24px" />
      </button>
      <div className={styles["popup-content"]}>
        <p>How's this page?</p>
        <div className={styles["popup-buttons"]}>
          <button
            type="button"
            onClick={() => handleFeedbackSelect("positive")}
            className={`${styles.button} ${styles.positive}`}
          >
            <Icon icon="mdi:thumbs-up" size="24px" />
          </button>
          <button
            type="button"
            onClick={() => handleFeedbackSelect("negative")}
            className={`${styles.button} ${styles.negative}`}
          >
            <Icon icon="mdi:thumbs-down" size="24px" />
          </button>
        </div>
      </div>
    </div>
  );
};

const Form = ({
  formName,
  url,
  feedbackType,
  setFeedbackType,
  comment,
  setComment,
  handleSubmit,
}) => (
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
        className={`${styles.button} ${feedbackType === "positive" ? styles.positive : ""}`}
      >
        <Icon icon="mdi:thumbs-up" size="48px" />
      </button>
      <button
        type="button"
        onClick={() => setFeedbackType("negative")}
        className={`${styles.button} negative ${feedbackType === "negative" ? styles.negative : ""}`}
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
      </div>
    )}

    {feedbackType && (
      <div className={styles.section}>
        <button className={styles.button} type="submit">
          <Icon icon="mdi:send" size="48px" />
        </button>
      </div>
    )}
  </form>
);
