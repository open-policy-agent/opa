import Mermaid from "@theme/Mermaid";

const logoPath = require("./logo.png").default;

const diagram = `
graph TD;
  Client -->|Request/Event| Service;
  Service -->|"Query<br/>(any JSON Value)"| OPA["<img src='${logoPath}' width='50' />"];
  OPA -->|"Decision<br/>(any JSON Value)"| Service;
  Policy["Policy (Rego)"] --> OPA;
  Data["Data (JSON)"] --> OPA;
`;

const OverviewDiagram = () => <Mermaid value={diagram} />;

export default OverviewDiagram;
