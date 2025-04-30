import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import React from "react";

import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import TabItem from "@theme/TabItem";
import Tabs from "@theme/Tabs";

import Card from "@site/src/components/Card";
import ImageCard from "@site/src/components/ImageCard";
import PlaygroundExample from "@site/src/components/PlaygroundExample";
import SideBySideColumn from "@site/src/components/SideBySide/Column";
import SideBySideContainer from "@site/src/components/SideBySide/Container";

const Index = (props) => {
  const title = "Open Policy Agent - Homepage";
  return (
    <Layout title={title}>
      <div style={{ display: "block", maxWidth: "75rem", width: "100%", margin: "0 auto" }}>
        <div style={{ display: "flex", flexWrap: "wrap", marginBottom: "2rem", padding: "1rem" }}>
          <div style={{ flex: "1", minWidth: "25rem" }}>
            <div
              style={{
                backgroundColor: "#e6f4ff",
                padding: "4rem 2rem",
                textAlign: "center",
                color: "#003366",
                borderRadius: "1rem",
                marginBottom: "2rem",
              }}
            >
              <img
                src={require("./assets/logo-text.png").default}
                alt="OPA Logo"
                style={{ width: "80%", marginBottom: "1rem", maxWidth: "30rem" }}
              />

              <h2
                style={{
                  fontWeight: "normal",
                  color: "#444",
                  marginTop: 0,
                  fontSize: "1.2rem",
                }}
              >
                OPA is a policy engine that streamlines policy management across your stack for improved development,
                security and audit capability.
              </h2>
            </div>
          </div>
          <div style={{ flex: "1.5", minWidth: "400px", padding: "0rem 1rem" }}>
            <PlaygroundExample dir={require.context("./_examples/admin")} />
          </div>
        </div>
      </div>

      <div style={{ display: "block", maxWidth: "60rem", width: "100%", margin: "0 auto" }}>
        <div style={{ margin: "0 auto", display: "flex", flexWrap: "wrap", gap: "1rem", justifyContent: "center" }}>
          {(() => {
            const logoContext = require.context("./assets/logos", false);
            return logoContext.keys().map((key) => {
              const logo = logoContext(key).default;
              const imageName = key.replace("./", "").split(".")[0];
              return (
                <div
                  key={key}
                  style={{
                    width: "6rem",
                    height: "6rem",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <img src={logo} alt={imageName} style={{ maxWidth: "80%", maxHeight: "80%" }} />
                </div>
              );
            });
          })()}
        </div>
      </div>

      <p style={{ textAlign: "center", fontSize: "0.9rem", marginBottom: "1rem" }}>Created by</p>

      <p
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: "0.5rem",
          flexWrap: "wrap",
          textAlign: "center",
          marginBottom: "0rem",
        }}
      >
        <a href="https://styra.com" target="_blank" rel="noopener noreferrer">
          <img
            src={require("./assets/styra.png").default}
            alt="Styra Logo"
            style={{ width: "6rem" }}
          />
        </a>
      </p>

      <p style={{ textAlign: "center", fontSize: "0.8rem", color: "#555", marginTop: "1rem" }}>
        OPA is now maintained by Styra and a large community of contributors.
      </p>

      <div style={{ display: "block", maxWidth: "75rem", width: "100%", margin: "0 auto" }}>
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: "2rem",
            margin: "2rem auto",
            width: "100%",
            justifyContent: "center",
            padding: "0 1rem",
          }}
        >
          <div
            style={{
              flex: "1 1 30rem",
              minWidth: "20rem",
              display: "flex",
              flexDirection: "column",
              gap: "2rem",
            }}
          >
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "6rem 1fr",
                gap: "1rem",
                alignItems: "center",
              }}
            >
              <img
                src={require("./assets/icons/productivity.png").default}
                alt="Productivity icon"
                style={{
                  display: "block",
                  width: "100%",
                  height: "auto",
                }}
              />
              <p style={{ margin: "0" }}>
                <strong>Developer Productivity:</strong>{" "}
                OPA helps teams focus on delivering business value by decoupling policy from application logic. Security
                & platform teams centrally manage shared policies, while developer teams extend them as needed within
                the policy system.
              </p>
            </div>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "6rem 1fr",
                gap: "1rem",
                alignItems: "center",
              }}
            >
              <img
                src={require("./assets/icons/performance.png").default}
                alt="Performance icon"
                style={{
                  display: "block",
                  width: "100%",
                  height: "auto",
                }}
              />
              <p style={{ margin: "0" }}>
                <strong>Performance:</strong>{" "}
                Rego, our domain-specific policy language, is built for speed. By operating on pre-loaded, in-memory
                data, OPA acts as a fast policy decision point for your applications.
              </p>
            </div>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "6rem 1fr",
                gap: "1rem",
                alignItems: "center",
              }}
            >
              <img
                src={require("./assets/icons/audit.png").default}
                alt="Audit icon"
                style={{
                  display: "block",
                  width: "100%",
                  height: "auto",
                }}
              />
              <p style={{ margin: "0" }}>
                <strong>Audit & Compliance:</strong>{" "}
                OPA generates comprehensive audit trails for every policy decision. This detailed history supports
                auditing and compliance efforts and enables decisions to be replayed for analysis or debugging.
              </p>
            </div>
          </div>

          <div
            style={{
              flex: "1 1 40rem",
              minWidth: "25rem",
              width: "100%",
              backgroundColor: "#e6f4ff",
              padding: "2rem",
              color: "#003366",
              borderRadius: "1rem",
            }}
          >
            <div
              style={{
                position: "relative",
                width: "100%",
                paddingBottom: "56.25%",
                height: 0,
                overflow: "hidden",
                marginBottom: "1rem",
              }}
            >
              <iframe
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: "100%",
                  border: "0.2rem solid #003366",
                }}
                src="https://www.youtube.com/embed/Icl0_b5Llqc?si=t4GZVXYI2-mfZFCe&controls=0&modestbranding=1&rel=0&showinfo=0"
                title="YouTube video player"
                frameBorder="0"
                allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                referrerPolicy="strict-origin-when-cross-origin"
              >
              </iframe>
            </div>
            <p
              style={{
                textAlign: "center",
                margin: "0 auto",
              }}
            >
              Interested to see more? Checkout the{" "}
              <a target="_blank" href="https://www.youtube.com/watch?v=XtA-NKoJDaI">
                Maintainer Track Session
              </a>{" "}
              from KubeCon.
            </p>
          </div>
        </div>

        <div
          style={{
            marginTop: "2rem",
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "center",
            gap: "1rem",
            padding: "0 1rem",
          }}
        >
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
            link_text: "Instant Join Slack",
          }, {
            title: "Contribute to OPA",
            note: "Get involved with our project",
            icon: require("./assets/github.png").default,
            link: useBaseUrl("/docs/contributing"),
            link_text: "Get started",
          }].map((cardItem, index) => (
            <div key={index} style={{ flex: "1 1 30%", minWidth: "250px", maxWidth: "400px" }}>
              <Card item={cardItem} />
            </div>
          ))}
        </div>
      </div>
    </Layout>
  );
};

export default Index;
