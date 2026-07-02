import React from "react";

import Heading from "@theme/Heading";

import Card from "../../components/Card";
import CardGrid from "../../components/CardGrid";
import StandaloneLayout from "../../components/StandaloneLayout";

const vendors = [
  {
    title: "Policy-as-Code Laboratories",
    icon: require.context("./assets/logos/paclabs.png").default,
    note:
      "Policy-as-Code Laboratories provides strategic planning and integration consulting for OPA and Rego across the PaC ecosystem (Cloud, Kubernetes, OpenShift, and legacy platforms).",
    link: "https://paclabs.io/opa_support.html?utm_source=opa&utm_content=opa-support",
    link_text: "Learn more",
    dateAdded: "2023-08-25",
  },
  {
    title: "DepKeep",
    icon: require.context("./assets/logos/depkeep.png").default,
    note:
      "DepKeep provides enterprise open-source support, maintenance, and security fixes for critical dependencies used in production systems. It helps teams keep open-source infrastructure secure, stable, and production-ready.",
    link: "https://depkeep.com/services/opa",
    link_text: "Learn more",
    dateAdded: "2026-06-10",
  },
];

const sortedVendors = [...vendors].sort((a, b) => a.dateAdded.localeCompare(b.dateAdded));

export default function Support() {
  return (
    <StandaloneLayout
      title="Support"
      description="Commercial Support Options for Open Policy Agent"
    >
      <Heading as="h1">Open Policy Agent Support</Heading>
      <p>
        Below is a list of companies that offer commercial support and other enterprise offerings for Open Policy Agent.
        Companies add themselves via pull request and listings are ordered by the date they were added.
      </p>
      <p className="margin-bottom--lg">
        This list is not vetted by the OPA project maintainers and inclusion is not an endorsement. Evaluate offerings
        to find the right fit for your needs.
      </p>

      <CardGrid justifyCenter={false}>
        {sortedVendors.map((item, idx) => <Card key={idx} item={item} />)}
      </CardGrid>
    </StandaloneLayout>
  );
}
