import React from "react";

import BuiltinTable from "../BuiltinTable";
import StandaloneLayout from "../StandaloneLayout";

function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

export default function BuiltinCategoryPage({ category, dir }) {
  if (!dir) {
    return (
      <div>
        <h2 style={{ marginBottom: "1.4rem" }}>{capitalize(category)}</h2>
        <BuiltinTable category={category} />
      </div>
    );
  }
  let source_files = dir.keys().reduce((acc, key) => {
    if (!key.startsWith(`./${category}/`)) {
      return acc;
    }
    let fileName = key.replace(`./${category}/`, "");
    if (!fileName.includes(".")) {
      return acc;
    }
    console.log(dir(key));
    if (fileName.endsWith(".json")) {
      acc[fileName] = dir(key);
      return acc;
    }
    acc[fileName] = dir(key).default;
    return acc;
  }, {});
  const config = source_files["config.json"];
  const contents = source_files["contents.md"];
  const metadata = source_files["metadata.json"];

  const categoryName = config?.name ?? capitalize(category);
  const customLinks = config?.customLinks ?? false;
  console.log(contents ? contents() : "");

  return (
    <StandaloneLayout>
      <div>
        <h2 style={{ marginBottom: "1.4rem" }}>{categoryName}</h2>
        <BuiltinTable category={category}>
          <div>
            {contents && contents()}
          </div>
        </BuiltinTable>
      </div>
    </StandaloneLayout>
  );
}
