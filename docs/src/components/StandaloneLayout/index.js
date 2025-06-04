import React from "react";

import Layout from "@theme/Layout";

import styles from "./styles.module.css";

/**
 * A layout component that replicates the styling of a standard MDX page.
 * This is used to maintain consistent styling with mdx pages like security.
 */
export default function StandaloneLayout({ title, description, children }) {
    return (
        <Layout title={title} description={description}>
            <main className="container container--fluid margin-vert--lg">
                <div className="row">
                    <div className={`col ${styles.col}`}>
                        <div className={styles.container}>
                            <article>
                                {children}
                            </article>
                        </div>
                    </div>
                </div>
            </main>
        </Layout>
    );
}
