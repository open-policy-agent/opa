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

let _ecosystemPagesCache = null;
export async function loadEcosystemPages(siteDir) {
  if (_ecosystemPagesCache) return _ecosystemPagesCache;

  const entryGlob = path.resolve(path.join(siteDir, "src/data/ecosystem/entries/*.md"));
  const logoGlobRoot = path.resolve(path.join(siteDir, "static/img/ecosystem/logos"));

  const pages = await loadPages(entryGlob);

  for (const id in pages) {
    const logoFiles = await new Promise((resolve, reject) => {
      glob(`${logoGlobRoot}/${id}*`, (err, matches) => {
        if (err) reject(err);
        else resolve(matches);
      });
    });

    const logoPath = logoFiles.length > 0
      ? `/img/ecosystem/logos/${path.basename(logoFiles[0])}`
      : "/img/logo.png";

    pages[id].logo = logoPath;
  }

  _ecosystemPagesCache = pages;
  return pages;
}
