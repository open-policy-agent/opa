import React, { useState } from "react";

import Link from "@docusaurus/Link";

import rules from "@generated/regal/default/rules.json";

import styles from "./styles.module.css";

export default function RulesTable({ category }) {
  const [searchQuery, setSearchQuery] = useState("");

  let predicates = [];

  let basePath = "./rules/";
  if (category !== undefined && category !== "") {
    predicates.push((rule) => rule.id.startsWith(category + "/"));
    basePath = "./";
  }

  if (searchQuery !== "") {
    predicates.push((rule) => rule.id.includes(searchQuery.toLowerCase()));
  }

  const filteredRules = rules.filter(rule => {
    if (predicates.length === 0) return true;

    return predicates.map((predicate) => predicate(rule))
      .every((e) => e == true);
  });

  return (
    <div className={styles.container}>
      <div className={styles.searchContainer}>
        <input
          type="text"
          placeholder="Search entries..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className={styles.searchInput}
        />
      </div>

      {filteredRules.length === 0
        ? <p>No matching rules</p>
        : (
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Rule</th>
                <th>Summary</th>
              </tr>
            </thead>
            <tbody>
              {filteredRules.map((rule) => {
                return (
                  <tr key={rule.id}>
                    <td>
                      <Link to={basePath + rule.id}>{rule.id}</Link>
                    </td>
                    <td>{rule.summary}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
    </div>
  );
}
