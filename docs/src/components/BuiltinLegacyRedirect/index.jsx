import { Redirect, useLocation } from "@docusaurus/router";
import React from "react";

// This component redirects the old built-in links to the new category pages,
// https://www.openpolicyagent.org/docs/policy-reference#builtin-comparison-equal leads to
// https://www.openpolicyagent.org/docs/policy-reference/builtins/comparison#builtin-comparison-equal
// Should be removed around version 1.12 or 1.13, only used in docs/policy-reference/index.md
export default function BuiltinLegacyRedirect({}) {
  const loc = useLocation();
  const hash = loc.hash.replace("#", "").split("-");
  if (hash.length == 3 && hash[0] == "builtin") {
    const newLoc = `/docs/policy-reference/builtins/${hash[1]}${loc.hash}`;
    return <Redirect to={newLoc} />;
  }
  return <div />;
}
