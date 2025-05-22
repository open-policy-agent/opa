import Mermaid from "@theme/Mermaid";

const logoPath = require("./logo.png").default;

const diagram = `
graph LR
  subgraph CP[Control Plane]
    Monitoring
    Logging
    Config
    Bundles
  end

  OPA["<img src='${logoPath}' width='30' height='30' /><br/>OPA"];
  Service["Service"] --- OPA

  OPA -->|Status| Monitoring
  OPA -->|Decisions| Logging
  Bundles -->|Bundles| OPA
  Config -->|Discovery<br/>Bundles| OPA

`;

const ControlPlaneDiagram = () => <Mermaid value={diagram} />;

export default ControlPlaneDiagram;
