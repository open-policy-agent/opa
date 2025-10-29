import fs from "fs/promises";
import { matter } from "md-front-matter";
import path from "path";

import { glob } from "glob";

export async function loadPages(globPattern) {
  const files = await glob(globPattern);

  const pages = await files.reduce(async (accPromise, filePath) => {
    const acc = await accPromise;
    const content = await fs.readFile(filePath, "utf-8");
    const parsed = matter(content);

    const id = path.parse(filePath).name;

    acc[id] = {
      ...parsed.data,
      content: parsed.content,
      filePath,
      id,
    };

    return acc;
  }, Promise.resolve({}));

  return pages;
}
