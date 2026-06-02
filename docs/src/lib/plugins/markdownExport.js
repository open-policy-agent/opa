import * as cheerio from "cheerio";
import fs from "fs/promises";
import { glob } from "glob";
import path from "path";
import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";
import { COPY_EXCLUDE_SELECTORS, createMarkdownConverter } from "../pageMarkdown.js";

export async function markdownExportPlugin(_context, _options) {
  return {
    name: "markdown-export",

    async postBuild({ outDir }) {
      const convert = createMarkdownConverter(TurndownService, gfm);
      const htmlFiles = await glob(path.join(outDir, "**/*.html"));
      let count = 0;

      await Promise.all(htmlFiles.map(async (htmlPath) => {
        const html = await fs.readFile(htmlPath, "utf-8");
        const $ = cheerio.load(html);

        const markdownDiv = $("article .theme-doc-markdown");
        if (markdownDiv.length === 0) return;

        for (const sel of COPY_EXCLUDE_SELECTORS) {
          markdownDiv.find(sel).remove();
        }

        const innerHTML = markdownDiv.html();
        if (!innerHTML) return;

        const markdown = convert(innerHTML);
        if (!markdown) return;

        const mdPath = htmlPath.replace(/\.html$/, ".md");
        await fs.writeFile(mdPath, markdown, "utf-8");
        count++;
      }));

      console.log(`[markdown-export] Wrote ${count} .md files to ${outDir}`);
    },
  };
}
