import React from "react";

import BrowserOnly from "@docusaurus/BrowserOnly";

import ParamContext from "../ParamContext";

// ParamProvider is a component that creates a context for managing editable
// params on a documentation page.
const ParamProvider = ({ children, initialParams }) => {
  return (
    <BrowserOnly fallback={<div>Loading parameters...</div>}>
      {() => {
        const [params, setParams] = React.useState(initialParams || {});

        const updateParam = (key, value) => {
          setParams((prevParams) => ({
            ...prevParams,
            [key]: value,
          }));
        };

        return (
          <ParamContext.Provider value={{ params, updateParam }}>
            {children}
          </ParamContext.Provider>
        );
      }}
    </BrowserOnly>
  );
};

export default ParamProvider;
