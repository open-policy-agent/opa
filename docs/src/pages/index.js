import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import React from "react";

import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";
import ThemedImage from "@theme/ThemedImage";

import Card from "@site/src/components/Card";
import CardGrid from "@site/src/components/CardGrid";
import PlaygroundExample from "@site/src/components/PlaygroundExample";

import StyraLogo from "./assets/styra.svg";
import styles from "./index.module.css";

const Index = (props) => {
  const title = "Open Policy Agent - Homepage";
  return (
    <Layout title={title}>
      <div className={styles.container}>
        <div className={styles.heroContainer}>
          <div className={styles.heroLeft}>
            <div className={styles.heroContent}>
              <ThemedImage
                alt="OPA Logo"
                className={styles.logo}
                sources={{
                  light: require("./assets/logo-text-light.png").default,
                  dark: require("./assets/logo-text-dark.png").default,
                }}
              />

              <h2 className={styles.subtitle}>
                OPA is a policy engine that streamlines policy management across your stack for improved development,
                security and audit capability.
              </h2>
            </div>
          </div>
          <div className={styles.heroRight}>
            <PlaygroundExample dir={require.context("./_examples/admin")} />
          </div>
        </div>
      </div>

      <div className={styles.container}>
        <div className={styles.logoContainer}>
          {(() => {
            const logoContext = require.context("./assets/logos", false);
            const companyNames = Array.from(
              new Set(
                logoContext.keys().map((key) => {
                  return key
                    .replace("./", "")
                    .replace("-light", "")
                    .replace("-dark", "")
                    .replace(".svg", "");
                }),
              ),
            );
            return companyNames.map((name) => {
              const logoLight = logoContext(`./${name}-light.svg`).default;
              const logoDark = logoContext(`./${name}-dark.svg`).default;
              return (
                <div key={name} className={styles.logoWrapper}>
                  <ThemedImage
                    alt={`${name} logo`}
                    className={styles.companyLogo}
                    sources={{
                      light: logoLight,
                      dark: logoDark,
                    }}
                  />
                </div>
              );
            });
          })()}
        </div>
      </div>

      <p className={styles.cncfContainer}>
        Open Policy Agent is a <a href="https://www.cncf.io/">Cloud Native Computing Foundation</a> Graduated project.

        <div className={styles.cncfLogo}>
          <img src={useBaseUrl("img/footer/cncf-light.svg")} alt="CNCF Logo" class="light-only" />
          <img src={useBaseUrl("img/footer/cncf-dark.svg")} alt="CNCF Logo" class="dark-only" />
        </div>
      </p>

      <div className={styles.container}>
        <div className={styles.featuresContainer}>
          <div className={styles.featuresLeft}>
            <div className={styles.featureItem}>
              <img
                src={require("./assets/icons/productivity.png").default}
                alt="Productivity icon"
                className={styles.featureIcon}
              />
              <p className={styles.featureText}>
                <strong>Developer Productivity:</strong>{" "}
                OPA helps teams focus on delivering business value by decoupling policy from application logic. Security
                & platform teams centrally manage shared policies, while developer teams extend them as needed within
                the policy system.
              </p>
            </div>

            <div className={styles.featureItem}>
              <img
                src={require("./assets/icons/performance.png").default}
                alt="Performance icon"
                className={styles.featureIcon}
              />
              <p className={styles.featureText}>
                <strong>Performance:</strong>{" "}
                Rego, our domain-specific policy language, is built for speed. By operating on pre-loaded, in-memory
                data, OPA acts as a fast policy decision point for your applications.
              </p>
            </div>

            <div className={styles.featureItem}>
              <img
                src={require("./assets/icons/audit.png").default}
                alt="Audit icon"
                className={styles.featureIcon}
              />
              <p className={styles.featureText}>
                <strong>Audit & Compliance:</strong>{" "}
                OPA generates comprehensive audit trails for every policy decision. This detailed history supports
                auditing and compliance efforts and enables decisions to be replayed for analysis or debugging.
              </p>
            </div>
          </div>

          <div className={styles.videoContainer}>
            <div className={styles.videoBackground}>
              <div className={styles.videoWrapper}>
                <iframe
                  className={styles.video}
                  src="https://www.youtube.com/embed/Icl0_b5Llqc?si=t4GZVXYI2-mfZFCe&controls=0&modestbranding=1&rel=0&showinfo=0"
                  title="YouTube video player"
                  frameBorder="0"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  referrerPolicy="strict-origin-when-cross-origin"
                >
                </iframe>
              </div>
              <p className={styles.videoText}>
                Interested to see more? Checkout the{" "}
                <a target="_blank" href="https://www.youtube.com/watch?v=XtA-NKoJDaI">
                  Maintainer Track Session
                </a>{" "}
                from KubeCon.
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className={styles.container}>
        <div className={styles.contentSection}>
          <Heading as="h2" style={{ marginBottom: "1rem" }}>
            Context-aware, Expressive, Fast, Portable
          </Heading>

          <p>
            OPA is a general-purpose policy engine that unifies policy enforcement across the stack. OPA provides a
            high-level declarative language that lets you specify policy for a wide range of use cases. You can use OPA
            to enforce policies in applications, proxies, Kubernetes, CI/CD pipelines, API gateways, and more.
          </p>

          <Tabs
            defaultValue="app"
            values={[
              { label: "API", value: "app" },
              { label: "Envoy", value: "envoy" },
              { label: "Kubernetes", value: "k8s" },
              { label: "GenAI", value: "ai" },
            ]}
          >
            <TabItem value="app">
              <PlaygroundExample dir={require.context("./_examples/app")} />
            </TabItem>
            <TabItem value="envoy">
              <PlaygroundExample dir={require.context("./_examples/envoy")} />
            </TabItem>
            <TabItem value="k8s">
              <PlaygroundExample dir={require.context("./_examples/k8s")} />
            </TabItem>
            <TabItem value="ai">
              <PlaygroundExample dir={require.context("./_examples/ai")} />
            </TabItem>
          </Tabs>
        </div>
      </div>
      <div className={styles.container}>
        <CardGrid>
          {[{
            title: "Rego Playground",
            note: "Write your first Rego Policy",
            icon: require("./assets/logo.png").default,
            link: "https://play.openpolicyagent.org/",
            link_text: "Play with Rego",
          }, {
            title: "OPA Slack Community",
            note: "Talk to other users and maintainers",
            icon: require("./assets/slack.png").default,
            link: "https://slack.openpolicyagent.org/",
            link_text: "Join us on Slack",
          }, {
            title: "Contribute to OPA",
            note: "Get involved with our project",
            icon: require("./assets/github.png").default,
            link: useBaseUrl("/docs/contributing"),
            link_text: "Get started",
          }].map((cardItem, index) => <Card key={index} item={cardItem} />)}
        </CardGrid>
      </div>
    </Layout >
  );
};

export default Index;
