import fs from "fs/promises";

const { glob } = require("glob");

export async function loadRules() {
  const rootPath = "projects/regal/rules";

  const summaryMarker = "**Summary**:";

  const files = await glob(rootPath + "/*/*.md");

  const rules = await files
    .filter(file => file !== "index.md")
    .reduce(async (accPromise, filePath) => {
      const acc = await accPromise;
      const content = await fs.readFile(filePath, "utf-8");

      const summary = (content.split("\n")
        .filter(line => line.includes(summaryMarker))
        .find(_ => true) || "").replace(summaryMarker, "").trim();

      // index and deprecated pages
      if (summary === "") return acc;

      const id = filePath.replace(rootPath + "/", "").replace(".md", "");

      acc.push({
        filePath,
        content,
        id,
      });

      return acc;
    }, Promise.resolve([]));

  return rules.sort((a, b) => a.id - b.id);
}
