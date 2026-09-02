import builtins from "@generated/builtin-data/default/builtins.json";
import React, { useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";

import BuiltinCompatCells, {
  BuiltinCompatHeadings,
  builtinCompatTargets,
  builtinImplementations,
} from "../BuiltinCompat";
import styles from "./styles.module.css";

// Every "only" filter in the search bar is the same toggle: grey when off, green when on.
const toggleClass = (on) => `${styles.filterToggle} ${on ? styles.filterToggleOn : ""}`;

export default function BuiltinSearch({ entryLimit, alwaysShow, elementId }) {
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("");
  const [selectedImplementations, setSelectedImplementations] = useState([]);
  const [isWasm, setIsWasm] = useState(false);
  const implementations = builtinImplementations();
  const isFiltered = isWasm || searchTerm || selectedCategory
    || selectedImplementations.length > 0 || alwaysShow;
  const toggleImplementation = (id) =>
    setSelectedImplementations((selected) =>
      selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id]
    );
  const displayLimit = entryLimit;
  const allRows = useMemo(() => {
    return (Object.keys(builtins._categories).filter((a) => (a != "internal"))
      .map((category) => {
        return builtins._categories[category]
          .filter((builtin) => ((!builtins[builtin].deprecated ?? false) && (builtins[builtin])))
          .map((builtin) => {
            const fn = builtins[builtin];

            const anchor = `/docs/policy-reference/builtins/${category}#builtin-${category}-${
              builtin.replaceAll(".", "")
            }`;

            const isInfix = !!fn.infix;
            const isRelation = !!fn.relation;
            const args = fn.args || [];
            const result = fn.result || {};

            const signature = isInfix
              ? `${args[0]?.name || "x"} ${fn.infix} ${args[1]?.name || "y"}`
              : isRelation
              ? `${builtin}(${args.map((a) => a.name).join(", ")}, ${result.name})`
              : `${result.name || "result"} := ${builtin}(${args.map((a) => a.name).join(", ")})`;
            return {
              name: builtin,
              category: category,
              wasm: fn.wasm,
              implementations: fn.implementations || {},
              introduced: fn.introduced,
              infix: isInfix,
              signature: signature,
              anchor: anchor,
              versions: fn.available,
            };
          });
      })).reduce((a, b) => a.concat(b), []); // builds and flattens the function list
  });
  const filteredRows = allRows.filter((row) => (
    (row.name.toLowerCase().includes(searchTerm.toLowerCase())
      || row.name.replace(".", "").toLowerCase().includes(searchTerm.toLowerCase())
      || (row.signature.includes(searchTerm) && row.infix)) // filter name
    && (!selectedCategory || row.category == selectedCategory) // filter category
    && (!isWasm || row.wasm) // filter wasm
    // filter implementations: every selected one must support the builtin, so selecting more
    // than one narrows to the builtins they have in common
    && selectedImplementations.every((id) => id in row.implementations)
    && isFiltered // ensures that at least one criteria is being filtered on
  ));
  return (
    <>
      <div className={styles.searchBar}>
        <select
          title="Filter Catgeory"
          value={selectedCategory}
          onChange={(e) => setSelectedCategory(e.target.value)}
        >
          <option value="" key="0">All Categories</option>
          {Array.from(Object.keys(builtins._categories).entries()).map((
            mod,
          ) => (mod[1] != "internal" && <option key={mod[0] + 1} value={mod[1]}>{mod[1]}</option>))}
        </select>
        <input
          type="text"
          title="Function Name"
          placeholder="Search built-ins..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <span
          className={toggleClass(isWasm)}
          onClick={(e) => setIsWasm(!isWasm)}
        >
          {isWasm ? "✓" : "✗"} Wasm Only
        </span>
        {implementations.map((impl) => (
          <span
            key={impl.id}
            className={toggleClass(selectedImplementations.includes(impl.id))}
            onClick={() => toggleImplementation(impl.id)}
          >
            {selectedImplementations.includes(impl.id) ? "✓" : "✗"} {impl.label} Only
          </span>
        ))}
      </div>
      {isFiltered && renderResults(filteredRows, entryLimit)}
    </>
  );
}

function renderResults(filteredRows, entryLimit) {
  if (filteredRows.length == 0) return <p>No matches</p>;
  if (!entryLimit || filteredRows.length <= entryLimit) return (renderTable(filteredRows));
  return <p>Currently {filteredRows.length} matches, keep typing to narrow it down</p>;
}

function renderTable(filteredRows) {
  return (
    <table style={{ width: "100%", tableLayout: "fixed" }}>
      <colgroup>
        <col />
        <col style={{ width: "100%" }} />
        {builtinCompatTargets().map((target) => <col key={target.key} style={{ width: "4rem" }} />)}
      </colgroup>
      <thead>
        <tr>
          <th>Category</th>
          <th>Name</th>
          <BuiltinCompatHeadings />
        </tr>
      </thead>
      <tbody>
        {filteredRows.map((row) => (renderRow(row)))}
      </tbody>
    </table>
  );
}

function renderRow(row) {
  return (
    <tr key={row.anchor}>
      <td>
        <span className={styles.categoryTag}>
          {row.category}
        </span>
      </td>
      <td>
        <a href={`${row.anchor}`}>{row.name}</a>
        <br />
        <code>{row.signature}</code>
      </td>
      <BuiltinCompatCells fn={row} />
    </tr>
  );
}
