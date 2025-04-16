import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React from "react";

import Card from "../components/Card";

const vendors = [
  {
    title: "Styra",
    icon: "/img/support-logos/styra.png",
    note:
      "Styra is the original creator of Open Policy Agent and provides support, professional services, training, and enterprise products.",
    link:
      "https://www.styra.com/styra-opa-support?utm_medium=partner&utm_source=opa&utm_campaign=dmge&utm_content=opa-support",
    link_text: "Learn more",
  },
  {
    title: "Policy-as-Code Laboratories",
    icon: "/img/support-logos/paclabs.png",
    note:
      "Policy-as-Code Laboratories provides strategic planning and integration consulting for OPA and Rego across the PaC ecosystem (Cloud, Kubernetes, OpenShift, and legacy platforms).",
    link: "https://paclabs.io/opa_support.html?utm_source=opa&utm_content=opa-support",
    link_text: "Learn more",
  },
];

export default function Support() {
  return (
    <Layout title="Support" description="Commercial Support Options for Open Policy Agent">
      <div className="container margin-vert--lg">
        <Heading as="h1">Open Policy Agent Support</Heading>
        <p className="margin-bottom--lg">
          Below is a list of companies that offer commercial support and other enterprise offerings for Open Policy
          Agent.
        </p>

        <div style={{ display: "flex", flexWrap: "wrap", gap: "1.5rem" }}>
          {vendors.map((item, idx) => <Card key={idx} item={item} />)}
        </div>
      </div>
    </Layout>
  );
}
