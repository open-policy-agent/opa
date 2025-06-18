import React from "react";
import BuiltinCategoryPage from "./components/BuiltinCategoryPage";
const BuiltinsCategory = (props) => {
  const { category } = props.route.customData;
  return (
    <div>
      <BuiltinCategoryPage
        category={category}
        dir={require.context("../docs/policy-reference/_categories/")}
      />
    </div>
  );
};

export default BuiltinsCategory;
