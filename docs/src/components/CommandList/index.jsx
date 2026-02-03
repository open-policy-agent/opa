import React, { useState } from "react";

import CommandDoc from "../CommandDoc";

import styles from "./styles.module.css";

function filterCommandById(command, query) {
  const lowerQuery = query.toLowerCase();

  const matches = (text) => text && text.toLowerCase().includes(lowerQuery);

  const isMatch = matches(command.id);

  // (charlieegan3) we don't have child commands for now and it might need to adjusted when
  // we do, but I didn't want to ignore the fact they might exist in future.
  let matchedChildren = [];
  if (Array.isArray(command.children)) {
    matchedChildren = command.children
      .map(child => filterCommandById(child, query))
      .filter(Boolean);
  }

  if (isMatch || matchedChildren.length > 0) {
    return {
      ...command,
      children: matchedChildren.length > 0 ? matchedChildren : command.children,
    };
  }

  return null;
}

const CommandList = ({ commands }) => {
  const [search, setSearch] = useState("");

  const filtered = commands
    .map(cmd => filterCommandById(cmd, search))
    .filter(Boolean);

  const totalCommands = commands.length;
  const filteredCommands = filtered.length;

  return (
    <div>
      <input
        type="text"
        placeholder="Search by command name..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className={styles.searchInput}
      />

      {filteredCommands !== totalCommands && filteredCommands > 0 && (
        <p className={styles.searchResults}>
          Showing {filteredCommands}/{totalCommands} commands
          {filtered.length > 1 && "(" + filtered.map(cmd => cmd.id).join(", ") + ")"}
        </p>
      )}

      {filteredCommands === 0 ? <p className={styles.noResults}>No matching commands found.</p> : (
        filtered.map((cmd, idx) => <CommandDoc key={cmd.id || idx} command={cmd} />)
      )}
    </div>
  );
};

export default CommandList;
