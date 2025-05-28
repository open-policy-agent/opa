import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";

import Card from "../../components/Card";
import CardGrid from "../../components/CardGrid";

const communityData = {
  title: "Community",
  intro: `Since its launch in 2016, Open Policy Agent has steadily gained momentum as
  the de facto approach for establishing authorization policies across cloud native environments.
  Its remarkable growth and adoption is due in no small part to the amazing
  community that has grown up right alongside it.
  Leverage this list of community resources to maximize the value OPA can provide!`,
  sections: [
    {
      title: "Discuss OPA",
      items: [
        {
          title: "OPA Slack",
          icon: require.context("./assets/logos/slack.png").default,
          note: `Primary channel for community support and OPA maintainer discussions.
Join #help for support.`,
          link: "https://slack.openpolicyagent.org/",
          link_text: "Join us on Slack",
        },
        {
          title: "GitHub",
          icon: require.context("./assets/logos/github.png").default,
          note: `Get involved with OPA development; request a feature, file a bug,
or view the code.`,
          link: "https://github.com/open-policy-agent",
          link_text: "Visit OPA on GitHub",
        },
        {
          title: "OPA Knowledge Base",
          icon: require.context("./assets/logos/github-discussions.png").default,
          note: `Community powered support for OPA and Rego. Ask questions about writing
Rego files, implementing OPA, or share the configurations you are working on.`,
          link: "https://github.com/open-policy-agent/community/discussions",
          link_text: "Ask a Question",
        },
        {
          title: "Stack Overflow",
          icon: require.context("./assets/logos/stack-overflow.png").default,
          note: `Ask the global developer community questions about OPA with the tag #open-policy-agent`,
          link: "https://stackoverflow.com/questions/tagged/open-policy-agent",
          link_text: "Ask a Question",
        },
        {
          title: "LinkedIn",
          icon: require.context("./assets/logos/linkedin.png").default,
          note: `News about OPA and events where OPA appears.`,
          link: "https://www.linkedin.com/company/81893943",
          link_text: "Connect with Us",
        },
      ],
    },
    {
      title: "Learning Resources",
      items: [
        {
          title: "Styra Academy",
          icon: require.context("./assets/logos/styra-academy.png").default,
          note: `Learning portal with courses on OPA and Rego.`,
          link: "https://academy.styra.com",
          link_text: "Visit Styra Academy",
        },
        {
          title: "Awesome OPA",
          icon: require.context("./assets/logos/opa.png").default,
          note: `Curated list of OPA links and resources.`,
          link: "https://github.com/StyraInc/awesome-opa",
          link_text: "Visit Awesome OPA",
        },
      ],
    },
  ],
};

function Section({ section }) {
  return (
    <div style={{ marginBottom: "2rem" }}>
      <Heading as="h2">{section.title}</Heading>
      <CardGrid justifyContent="left">
        {section.items.map((item, idx) => <Card key={idx} item={item} />)}
      </CardGrid>
    </div>
  );
}

export default function CommunityPage() {
  const { title, intro, sections } = communityData;

  return (
    <Layout title={title} description="OPA Community Resources">
      <div className="container margin-vert--lg">
        <Heading as="h1">{title}</Heading>
        <p style={{ fontSize: "1.1rem", maxWidth: 700 }}>{intro}</p>

        {sections.map((section, idx) => <Section key={idx} section={section} />)}
      </div>
    </Layout>
  );
}
