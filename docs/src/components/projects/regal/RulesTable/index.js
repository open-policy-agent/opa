import Link from "@docusaurus/Link";
import React, { useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";
import styles from "./styles.module.css";

import rules from "@generated/regal/default/rules.json";

export default function RegalRulesTable({ category = "" }) {
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("");

  const preselectedCategory = category || null;

  const effectiveCategory = preselectedCategory || selectedCategory;

  const allRows = useMemo(() => {
    return Object.values(rules)
      .map((rule) => {
        const filePathMatch = rule.filePath.match(/\/regal\/rules\/([^/]+)\//);
        const pathCategory = filePathMatch ? filePathMatch[1] : "unknown";

        const content = rule.content || "";

        const summaryMatch = content.match(/\*\*Summary\*\*:\s*(.*?)\n/);
        const typeMatch = content.match(/\*\*Type\*\*:\s*(.*?)\n/);
        const fixableMatch = content.match(/\*\*Automatically fixable\*\*:\s*\[(.*?)\]\((.*?)\)/);

        const summaryParts = [];

        const summary = summaryMatch?.[1]?.trim();
        if (summary) summaryParts.push(summary);

        const type = typeMatch?.[1]?.trim();
        if (type) summaryParts.push(`**Type**: ${type}`);

        if (fixableMatch) {
          const label = fixableMatch[1];
          const link = fixableMatch[2];
          summaryParts.push(`**Automatically fixable**: [${label}](${link})`);
        }

        if (summaryParts.length === 0) return null;

        const contentCategoryMatch = content.match(/\*\*Category\*\*:\s*(.*?)\n/);

        return {
          pathCategory,
          displayCategory: contentCategoryMatch?.[1] || pathCategory,
          id: rule.id,
          content,
          summary: summaryParts.join("\n\n"),
        };
      })
      .filter(Boolean);
  }, []);

  const categories = useMemo(() => {
    const set = new Set(allRows.map((r) => r.pathCategory));
    return Array.from(set).sort();
  }, [allRows]);

  const filteredRows = useMemo(() => {
    const term = searchTerm.toLowerCase();
    return allRows
      .map((row) => {
        if (!term) return { ...row, contentOnlyMatch: false };

        const idMatch = row.id.toLowerCase().includes(term);
        const categoryMatch = row.displayCategory.toLowerCase().includes(term);
        const contentMatch = row.content.toLowerCase().includes(term);

        const contentOnlyMatch = contentMatch && !idMatch && !categoryMatch;
        const matches = idMatch || categoryMatch || contentMatch;

        return matches ? { ...row, contentOnlyMatch } : null;
      })
      .filter(Boolean)
      .filter((row) => effectiveCategory ? row.pathCategory === effectiveCategory : true)
      .sort((a, b) => a.displayCategory.localeCompare(b.displayCategory));
  }, [allRows, searchTerm, effectiveCategory]);

  return (
    <>
      <div className={styles.controls}>
        {!preselectedCategory && (
          <select
            className={styles.select}
            value={selectedCategory}
            onChange={(e) => setSelectedCategory(e.target.value)}
          >
            <option value="">All Categories</option>
            {categories.map((cat) => (
              <option key={cat} value={cat}>
                {cat.charAt(0).toUpperCase() + cat.slice(1)}
              </option>
            ))}
          </select>
        )}
        <input
          className={styles.input}
          type="text"
          placeholder="Search rules..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
      </div>

      {filteredRows.length === 0 && (
        <div>
          <p>No rules found.</p>
        </div>
      )}

      {filteredRows.length > 0 && (
        <table className={styles.table}>
          <thead>
            <tr>
              {!preselectedCategory && <th className={styles.th}>Category</th>}
              <th className={styles.th}>ID</th>
              <th className={styles.th}>Summary</th>
            </tr>
          </thead>
          <tbody>
            {filteredRows.map((row) => (
              <tr key={row.id}>
                {!preselectedCategory && (
                  <td className={styles.categoryCell}>
                    {row.displayCategory}
                  </td>
                )}
                <td className={styles.idCell}>
                  <Link to={`/projects/regal/rules/${row.id}`}>
                    {row.id}
                  </Link>
                  {row.contentOnlyMatch && (
                    <span className={styles.contentMatchIndicator}>
                      Content Matches Search
                    </span>
                  )}
                </td>
                <td className={styles.summaryCell}>
                  <ReactMarkdown>{row.summary}</ReactMarkdown>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
