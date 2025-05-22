import Mermaid from "@theme/Mermaid";

const logoPath = require("./logo.png").default;

const diagram = `
graph TD;
  subgraph SB[<b>Service B</b>]
    style SB fill:none,stroke:none;
    direction LR
    subgraph SBNP[Node/Pod]
      style SBNP fill:none,stroke-dasharray: 7 5
      direction LR
      subgraph "Local OPA Instance"
        B_OPA["<img src='${logoPath}' width='30' height='30' /><br/>OPA"];
      end
      subgraph "App Instance"
        B_Service["Service Logic"] -->|HTTP Call| B_OPA
      end
    end
  end

  subgraph SA[<b>Service A</b>]
    style SA fill:none,stroke:none;
    direction LR
    subgraph SANP[Node/Pod]
      style SANP fill:none,stroke-dasharray: 7 5
      direction LR
      subgraph "Local OPA Instance"
        A_OPA["<img src='${logoPath}' width='30' height='30' /><br/>OPA"];
      end
      subgraph "App Instance"
        A_Service["Service Logic"] -->|HTTP Call| A_OPA
      end
    end
  end
`;

const DistributedDiagram = () => <Mermaid value={diagram} />;

export default DistributedDiagram;
