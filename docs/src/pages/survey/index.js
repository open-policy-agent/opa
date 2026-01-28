import React from "react";
import Heading from "@theme/Heading";
import Link from "@docusaurus/Link";
import Admonition from "@theme/Admonition";

import StandaloneLayout from "../../components/StandaloneLayout";
import QuestionComparison from "../../components/QuestionComparison";

import eventData from "@generated/survey-data/default/survey-event-data.json";
import questions from "@generated/survey-data/default/survey-questions.json";
import eventMetadata from "@generated/survey-data/default/survey-event-metadata.json";

export default function Survey() {
  // Get all unique question IDs from events
  const allQuestionIds = Object.values(eventData).flatMap(data =>
    Object.keys(data)
  );
  const uniqueQuestionIds = Array.from(new Set(allQuestionIds));

  // Filter for only featured questions
  const featuredQuestionIds = uniqueQuestionIds.filter(questionId => {
    const question = questions[questionId];
    return question && question.featured === true;
  });

  // Get sorted event slugs by year
  const eventSlugs = Object.keys(eventData).sort((a, b) => {
    const yearA = eventMetadata[a]?.year || 0;
    const yearB = eventMetadata[b]?.year || 0;
    return yearA - yearB;
  });

  // Prepare event links data for the intro paragraph
  const eventLinks = eventSlugs.map((slug, index) => ({
    slug,
    year: eventMetadata[slug]?.year || slug,
    separator: index > 0 && (index === eventSlugs.length - 1 ? ", and " : ", ")
  }));

  // Prepare featured questions data for rendering
  const featuredQuestionsData = featuredQuestionIds.map(questionId => ({
    questionId,
    question: questions[questionId],
    eventData,
    eventSlugs,
    eventMetadata
  }));

  return (
    <StandaloneLayout
      title="Survey"
      description="Open Policy Agent Survey Results"
    >
      <Heading as="h1">OPA Survey Results</Heading>

      <p>
        In this section, we outline results from the various OPA community surveys we have run in{" "}
        {eventLinks.map(({ slug, year, separator }) => (
          <React.Fragment key={slug}>
            {separator}
            <Link to={`/survey/${slug}`}>{year}</Link>
          </React.Fragment>
        ))}.
      </p>

      <p>
        The charts that follow make a number of comparisons from over the years
        where the same questions were asked.
      </p>

      <Heading as="h2">Annual Trends</Heading>

      <Admonition type="caution">
        <p>
          The following section is a work in progress effort to compare some trends in survey responses over the years we've run them. Please see the 2025 survey for the most recent results and complete question set.
        </p>
      </Admonition>

      {featuredQuestionsData.map(({ questionId, question, eventData, eventSlugs, eventMetadata }) => (
        <QuestionComparison
          key={questionId}
          questionId={questionId}
          question={question}
          eventData={eventData}
          eventSlugs={eventSlugs}
          eventMetadata={eventMetadata}
        />
      ))}
    </StandaloneLayout>
  );
}

