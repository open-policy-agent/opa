export const COPY_EXCLUDE_SELECTORS = [
  "[data-copy-exclude]",
  ".hash-link",
  ".buttonGroup",
  "[hidden]",
];

// Factory: takes TurndownService constructor and gfm plugin, returns a converter function.
// Works in both browser (dynamic import) and Node.js (static import).
export function createMarkdownConverter(TurndownService, gfm) {
  const turndown = new TurndownService({
    headingStyle: "atx",
    codeBlockStyle: "fenced",
  });

  turndown.use(gfm);

  // Always render tables as GFM, compressing multi-line cell content
  turndown.addRule("docusaurusTable", {
    filter(node) {
      return node.nodeName === "TABLE";
    },
    replacement(_content, node) {
      const rows = node.rows;
      if (!rows || rows.length === 0) return _content;

      const cellText = (cell) =>
        turndown.turndown(cell.innerHTML)
          .replace(/\n/g, " ")
          .replace(/\|/g, "\\|")
          .trim();

      const headerCells = Array.from(rows[0].cells).map(cellText);
      const lines = [];
      lines.push("| " + headerCells.join(" | ") + " |");
      lines.push("| " + headerCells.map(() => "---").join(" | ") + " |");

      for (let i = 1; i < rows.length; i++) {
        const cells = Array.from(rows[i].cells).map(cellText);
        lines.push("| " + cells.join(" | ") + " |");
      }

      return "\n\n" + lines.join("\n") + "\n\n";
    },
  });

  return function convertToMarkdown(innerHTML) {
    let markdown = turndown.turndown(innerHTML);
    return markdown
      .replace(/#{4,}\s*/g, "")
      .replace(/\n{3,}/g, "\n\n")
      .trim();
  };
}
