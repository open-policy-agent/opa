import React from "react";
import semver from "semver";

import versions from "@generated/versions-data/default/versions.json";

// EvergreenCodeBlock is a component that can perform some templating of code blocks
// based on data available in docusaurus but not in mdx files.
function EvergreenCodeBlock({ children }) {
  const currentVersion = versions.filter(semver.valid).sort(semver.rcompare)[0];

  const replacements = {
    "current_version": currentVersion,
    "current_version_docker": currentVersion.replace("v", ""),
    "current_version_docker_envoy": currentVersion.replace("v", "") + "-envoy-4",
    // TODO: Automate the updates of this value
    "current_version_kube_mgmt": "9.0.1",
  };

  // Function to recursively process React children and replace template variables
  const processChildren = (children) => {
    return React.Children.map(children, child => {
      if (typeof child === "string") {
        let processedText = child;

        Object.entries(replacements).forEach(([key, value]) => {
          const pattern = new RegExp(`\\{\\{\\s*${key}\\s*\\}\\}`, "g");
          processedText = processedText.replace(pattern, value);
        });

        return processedText;
      }

      if (React.isValidElement(child) && child.props.children) {
        return React.cloneElement(child, {
          ...child.props,
          children: processChildren(child.props.children),
        });
      }

      return child;
    });
  };

  return (
    <div>
      {processChildren(children)}
    </div>
  );
}

export default EvergreenCodeBlock;

