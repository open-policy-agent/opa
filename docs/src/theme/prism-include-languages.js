/**
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

/**
 * This file was swizzled to provide custom Rego language highlighting.
 *
 * To recreate this swizzle:
 * npx docusaurus swizzle @docusaurus/theme-classic prism-include-languages --eject
 *
 * Modifications:
 * - Add custom formatting for Rego language with string interpolation
 */

import siteConfig from "@generated/docusaurus.config";
import regoLanguage from "./prism-rego";

export default function prismIncludeLanguages(PrismObject) {
  const {
    themeConfig: { prism },
  } = siteConfig;
  const { additionalLanguages } = prism;

  globalThis.Prism = PrismObject;

  additionalLanguages.forEach((lang) => {
    // skip rego since we're using our custom implementation
    if (lang === "rego") {
      return;
    }

    try {
      require(`prismjs/components/prism-${lang}`);
    } catch (error) {
      console.warn(`Prism language '${lang}' not found.`);
    }
  });

  regoLanguage(PrismObject);

  delete globalThis.Prism;
}
