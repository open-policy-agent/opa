import React from "react";
// Using clsx to conditionally apply Docusaurus global styles
import clsx from "clsx";

import { useWindowSize } from "@docusaurus/theme-common";
import Layout from "@theme/Layout";
import TOC from "@theme/TOC";

import styles from "./styles.module.css";

/**
 * A layout component that replicates the styling of a standard MDX page.
 * This is used to maintain consistent styling with mdx pages like security.
 */
export default function StandaloneLayout({ title, description, children, toc = null }) {
  const windowSize = useWindowSize();

  // Determine if we should show the TOC
  const canRenderTOC = toc && toc.length > 0;
  const showDesktopTOC = canRenderTOC
    && (windowSize === "desktop" || windowSize === "ssr");

  return (
    <Layout title={title} description={description}>
      <main className="container container--fluid margin-vert--lg">
        <div className="row">
          <div
            className={clsx(
              "col",
              styles.col,
              canRenderTOC && styles.colWithTOC,
            )}
          >
            <div className={styles.container}>
              <article>
                {children}
              </article>
            </div>
          </div>

          {showDesktopTOC && (
            <div className="col col--3">
              <TOC
                toc={toc}
                minHeadingLevel={2}
                maxHeadingLevel={2}
              />
            </div>
          )}
        </div>
      </main>
    </Layout>
  );
}
