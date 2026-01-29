import React from 'react';
import Heading from '@theme/Heading';

import { getChartComponent } from '../charts';

import styles from './styles.module.css';

export default function QuestionComparison({
  questionId,
  question,
  eventData,
  eventSlugs,
  eventMetadata,
}) {
  const ChartComponent = getChartComponent(question.comparisonChartType);

  const chartData = eventSlugs.map(eventSlug => {
    const eventQuestionData = eventData[eventSlug][questionId];
    if (!eventQuestionData || !eventQuestionData.data) return null;

    const displayName = eventMetadata[eventSlug]?.year?.toString() || eventSlug;
    const dataPoint = { name: displayName };

    eventQuestionData.data.forEach(item => {
      dataPoint[item.label] = item.value;
    });

    return dataPoint;
  }).filter(Boolean);

  const allCategories = eventSlugs.flatMap(eventSlug => {
    const eventQuestionData = eventData[eventSlug][questionId];
    return (eventQuestionData && eventQuestionData.data) ? eventQuestionData.data.map(item => item.label) : [];
  });
  const categories = Array.from(new Set(allCategories));

  const allFootnotes = eventSlugs.flatMap(eventSlug => {
    const eventQuestionData = eventData[eventSlug][questionId];
    if (!eventQuestionData || !eventQuestionData.footnotes || eventQuestionData.footnotes.length === 0) {
      return [];
    }
    const year = eventMetadata[eventSlug]?.year?.toString() || eventSlug;
    return eventQuestionData.footnotes.map(footnote => `${year}: ${footnote}`);
  });

  return (
    <div className={styles.container}>
      <Heading as="h3">{question.title}</Heading>
      {ChartComponent && (
        <ChartComponent
          data={chartData}
          config={question.chartConfig}
          categories={categories}
        />
      )}
      {allFootnotes.length > 0 && (
        <div className={styles.footnotesContainer}>
          {allFootnotes.map((footnote, index) => (
            <p key={index} className={styles.footnote}>
              {footnote}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}
