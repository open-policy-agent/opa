import fs from "fs/promises";
import glob from "glob";
import { matter } from "md-front-matter";
import path from "path";

export async function loadPages(globPattern) {
  const files = await new Promise((resolve, reject) => {
    glob(globPattern, (err, matches) => {
      if (err) reject(err);
      else resolve(matches);
    });
  });

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
