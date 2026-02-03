import React, { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import Heading from "@theme/Heading";
import Link from "@docusaurus/Link";

import StandaloneLayout from "../components/StandaloneLayout";
import QuestionSingle from "../components/QuestionSingle";

import allEventData from "@generated/survey-data/default/survey-event-data.json";
import questions from "@generated/survey-data/default/survey-questions.json";
import eventMetadata from "@generated/survey-data/default/survey-event-metadata.json";

import styles from "./styles.module.css";

export default function SurveyEvent(props) {
  const { eventSlug } = props.route.customData;
  const eventData = allEventData[eventSlug];
  const metadata = eventMetadata[eventSlug];

  if (!eventData) {
    return (
      <StandaloneLayout
        title={`Survey`}
        description={`Open Policy Agent Survey Results`}
      >
        <p>
          <Link to="/survey">&larr; View all survey events</Link>
        </p>
        <Heading as="h1">Survey Results</Heading>
        <p>No survey data available for this event.</p>
      </StandaloneLayout>
    );
  }

  const questionIds = Object.keys(eventData);
  const pageTitle = metadata?.title || `Survey Results`;

  // Group questions by tags and sort by rank
  const groupedQuestions = useMemo(() => {
    const groups = questionIds.reduce((acc, questionId) => {
      const question = questions[questionId];
      const tag = question?.tags?.[0] || 'uncategorized';
      (acc[tag] = acc[tag] || []).push(questionId);
      return acc;
    }, {});

    Object.values(groups).forEach(group =>
      group.sort((a, b) => (questions[a]?.rank || 999) - (questions[b]?.rank || 999))
    );

    return groups;
  }, [questionIds, questions]);

  // Build tag metadata from metadata.sections
  const { tagOrder, tagDisplayNames, tagDescriptions } = useMemo(() => {
    const order = metadata?.sections?.map(s => s.tag) || Object.keys(groupedQuestions);
    const displayNames = {
      uncategorized: 'Other',
      ...metadata?.sections?.reduce((acc, s) => ({ ...acc, [s.tag]: s.title }), {})
    };
    const descriptions = metadata?.sections?.reduce((acc, s) =>
      s.description ? { ...acc, [s.tag]: s.description } : acc
    , {}) || {};

    return { tagOrder: order, tagDisplayNames: displayNames, tagDescriptions: descriptions };
  }, [metadata, groupedQuestions]);

  // Generate TOC items from tag sections
  const tocItems = useMemo(() => {
    return tagOrder
      .filter(tag => {
        const questionsInGroup = groupedQuestions[tag];
        return questionsInGroup && questionsInGroup.length > 0;
      })
      .map(tag => ({
        value: tagDisplayNames[tag] || tag,
        id: tag,
        level: 2,
      }));
  }, [tagOrder, groupedQuestions, tagDisplayNames]);

  return (
    <StandaloneLayout
      title={metadata?.title || `Survey`}
      description={metadata?.title || `Open Policy Agent Survey Results`}
      toc={tocItems}
    >
      <p>
        <Link to="/survey">&larr; View all survey events</Link>
      </p>
      <Heading as="h1">{pageTitle}</Heading>

      {metadata?.intro && (
        <p>
          {metadata.intro}
          {metadata?.blog && (
            <>
              {" "}These results were originally presented in the following{" "}
              <Link to={metadata.blog}>blog post</Link>.
            </>
          )}
        </p>
      )}

      {tagOrder.map(tag => {
        const questionsInGroup = groupedQuestions[tag];
        if (!questionsInGroup || questionsInGroup.length === 0) {
          return null;
        }

        const displayName = tagDisplayNames[tag] || tag;
        const description = tagDescriptions[tag];

        return (
          <div key={tag} className={styles.tagSection}>
            <Heading as="h2" id={tag} className={styles.tagHeading}>
              {displayName}
            </Heading>
            {description && (
              <div className={styles.tagDescription}>
                <ReactMarkdown>{description}</ReactMarkdown>
              </div>
            )}
            {questionsInGroup.map(questionId => (
              <QuestionSingle
                key={questionId}
                question={questions[questionId]}
                eventData={eventData[questionId]}
              />
            ))}
          </div>
        );
      })}
    </StandaloneLayout>
  );
}
