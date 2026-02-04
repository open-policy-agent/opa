import Heading from "@theme/Heading";
import React from "react";

import { getChartComponent } from "../charts";

import styles from "./styles.module.css";

export default function QuestionSingle({
  question,
  eventData,
}) {
  if (!question || !eventData) {
    return null;
  }

  const ChartComponent = getChartComponent(question.chartType);

  if (question.chartType === "text-list") {
    return (
      <div className={styles.container}>
        <Heading as="h3">{question.title}</Heading>
        {ChartComponent && (
          <ChartComponent
            intro={eventData.intro}
            bullets={eventData.bullets}
            config={question.chartConfig}
          />
        )}
      </div>
    );
  }

  const chartData = eventData.data.map(item => ({
    name: item.label,
    value: item.value,
  }));

  const categories = [];

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
      {eventData.footnotes && eventData.footnotes.length > 0 && (
        <div className={styles.footnotesContainer}>
          {eventData.footnotes.map((footnote, index) => (
            <p key={index} className={styles.footnote}>
              {footnote}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}
