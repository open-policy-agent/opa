import React from "react";

import Heading from "@theme/Heading";

import Card from "../../components/Card";
import CardGrid from "../../components/CardGrid";
import StandaloneLayout from "../../components/StandaloneLayout";

const vendors = [
  {
    title: "Styra",
    icon: require.context("./assets/logos/styra.png").default,
    note:
      "Styra is the original creator of Open Policy Agent and provides support, professional services, training, and enterprise products.",
    link:
      "https://www.styra.com/styra-opa-support?utm_medium=partner&utm_source=opa&utm_campaign=dmge&utm_content=opa-support",
    link_text: "Learn more",
  },
  {
    title: "Policy-as-Code Laboratories",
    icon: require.context("./assets/logos/paclabs.png").default,
    note:
      "Policy-as-Code Laboratories provides strategic planning and integration consulting for OPA and Rego across the PaC ecosystem (Cloud, Kubernetes, OpenShift, and legacy platforms).",
    link: "https://paclabs.io/opa_support.html?utm_source=opa&utm_content=opa-support",
    link_text: "Learn more",
  },
];

export default function Support() {
  return (
    <StandaloneLayout
      title="Support"
      description="Commercial Support Options for Open Policy Agent"
    >
      <Heading as="h1">Open Policy Agent Support</Heading>
      <p className="margin-bottom--lg">
        Below is a list of companies that offer commercial support and other enterprise offerings for Open Policy
        Agent.
      </p>

      <CardGrid justifyCenter={false}>
        {vendors.map((item, idx) => <Card key={idx} item={item} />)}
      </CardGrid>
    </StandaloneLayout>
  );
}
