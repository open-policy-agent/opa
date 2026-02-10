import fs from "fs/promises";
import { glob } from "glob";
import path from "path";

export async function loadEvents(globPattern) {
  const files = await glob(globPattern);

  const events = await files.reduce(async (accPromise, filePath) => {
    const acc = await accPromise;
    const content = await fs.readFile(filePath, "utf-8");
    const eventData = JSON.parse(content);

    const id = eventData.id || path.parse(filePath).name;

    acc[id] = {
      ...eventData,
      id,
      filePath,
    };

    return acc;
  }, Promise.resolve({}));

  return events;
}
