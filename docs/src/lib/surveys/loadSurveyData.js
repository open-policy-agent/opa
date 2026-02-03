import fs from "fs/promises";
import path from "path";
import { glob } from "glob";

/**
 * Load survey event data from JSON files
 * Structure: events/{event-slug}/{question-id}/data.json
 * Returns: { "2025-community-survey": { "question-id": { data: [...], footnotes: [] } }, ... }
 */
export async function loadSurveyEventData(globPattern) {
  const files = await glob(globPattern);
  const eventData = {};

  for (const filePath of files) {
    const content = await fs.readFile(filePath, "utf-8");
    const data = JSON.parse(content);

    // Extract event slug and question-id from path
    // Example: src/data/surveys/events/2025-community-survey/most-advanced-use-case/data.json
    const pathParts = filePath.split(path.sep);
    const eventsIndex = pathParts.findIndex(p => p === "events");

    if (eventsIndex === -1) continue;

    const eventSlug = pathParts[eventsIndex + 1];
    const questionId = pathParts[eventsIndex + 2];

    // Skip metadata.json files
    if (questionId === "metadata.json") continue;

    if (!eventData[eventSlug]) {
      eventData[eventSlug] = {};
    }

    eventData[eventSlug][questionId] = data;
  }

  return eventData;
}

/**
 * Load survey question metadata from JSON files
 * Structure: questions/{question-id}/data.json
 * Returns: { "question-id": { id, title, chartType, chartConfig } }
 */
export async function loadSurveyQuestions(globPattern) {
  const files = await glob(globPattern);
  const questions = {};

  for (const filePath of files) {
    const content = await fs.readFile(filePath, "utf-8");
    const data = JSON.parse(content);

    // Extract question-id from path
    // Example: src/data/surveys/questions/most-advanced-use-case/data.json
    const pathParts = filePath.split(path.sep);
    const questionsIndex = pathParts.findIndex(p => p === "questions");

    if (questionsIndex === -1) continue;

    const questionId = pathParts[questionsIndex + 1];

    questions[questionId] = data;
  }

  return questions;
}

/**
 * Load survey event metadata from JSON files
 * Structure: events/{event-slug}/metadata.json
 * Returns: { "2025-community-survey": { slug, title, year }, ... }
 */
export async function loadSurveyEventMetadata(globPattern) {
  const files = await glob(globPattern);
  const eventMetadata = {};

  for (const filePath of files) {
    const content = await fs.readFile(filePath, "utf-8");
    const data = JSON.parse(content);

    // Use slug from the data
    const slug = data.slug;
    eventMetadata[slug] = data;
  }

  return eventMetadata;
}
