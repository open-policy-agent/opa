import builtins from "@generated/builtin-data/default/builtins.json";
import React from "react";

import styles from "./styles.module.css";

// The Rego interpreters other than OPA itself, in the order they are recorded. Refreshed from
// each implementation's published capabilities by build/update-implementations.sh.
export function builtinImplementations() {
  return builtins._implementations || [];
}

// One column per place a builtin can be used, in the spirit of MDN's browser compatibility
// tables: OPA itself, Wasm-compiled policies, then each other Rego implementation. Implementations
// come from the recorded capabilities, so a new one appears as a new column with no code change.
export function builtinCompatTargets() {
  return [
    { key: "opa", label: "OPA" },
    { key: "wasm", label: "Wasm" },
    ...builtinImplementations().map((impl) => ({
      key: impl.id,
      label: impl.label,
      href: `https://github.com/${impl.repo}`,
      name: impl.repo.split("/").pop(),
      version: impl.version,
    })),
  ];
}

// Header cells for the compatibility columns.
export function BuiltinCompatHeadings() {
  return (
    <>
      {builtinCompatTargets().map((target) => (
        <th key={target.key} className={styles.heading}>
          {target.href
            ? (
              <a href={target.href} target="_blank" rel="noopener noreferrer">
                {target.label}
              </a>
            )
            : target.label}
        </th>
      ))}
    </>
  );
}

// support describes how one target covers one builtin: the version support began in where that is
// known, a tick where it is supported without a version, or a cross where it is not. A builtin
// left out of the Wasm module is not unavailable in the way a cross elsewhere is — the policy
// still evaluates if the host SDK answers the callback — so that case gets its own "SDK" state
// rather than reusing the cross for two different meanings.
function support(target, fn) {
  if (target.key === "opa") {
    const introduced = fn.introduced;
    // "edge" is unreleased, and v0.17.0 is the oldest capabilities file, so it means "at least
    // this old" rather than a release to link to.
    const linkable = introduced && introduced !== "edge" && introduced !== "v0.17.0";
    return {
      state: "yes",
      text: introduced,
      href: linkable ? `https://github.com/open-policy-agent/opa/releases/${introduced}` : null,
      title: `Available in OPA ${introduced}`,
    };
  }

  if (target.key === "wasm") {
    if (!fn.wasm) {
      return {
        state: "host",
        text: "SDK",
        title: "Not compiled into Wasm; must be provided by the host SDK",
      };
    }
    return {
      state: "yes",
      text: "✓",
      title: "Available in policies compiled to Wasm",
    };
  }

  const availability = fn.implementations || {};
  if (!(target.key in availability)) {
    return {
      state: "no",
      text: "✗",
      title: `Not available in ${target.name} ${target.version}`,
    };
  }

  const since = availability[target.key];
  return {
    state: "yes",
    text: since || "✓",
    title: since
      ? `Available in ${target.name} since ${since}`
      : `Available in ${target.name} ${target.version}`,
  };
}

const cellStyles = {
  yes: styles.supported,
  no: styles.unsupported,
  host: styles.hostProvided,
};

// Body cells for the compatibility columns of one builtin.
export default function BuiltinCompatCells({ fn }) {
  return (
    <>
      {builtinCompatTargets().map((target) => {
        const cell = support(target, fn);
        return (
          <td
            key={target.key}
            className={cellStyles[cell.state]}
            title={cell.title}
          >
            {cell.href
              ? (
                <a href={cell.href} target="_blank" rel="noopener noreferrer">
                  {cell.text}
                </a>
              )
              : cell.text}
          </td>
        );
      })}
    </>
  );
}
