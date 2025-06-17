import Mermaid from "@theme/Mermaid";

const logoPath = require("./logo.png").default;

const diagram = `
graph TD;
  subgraph LM[<b>Library Model</b>]
    style LM fill:none,stroke:none;
    direction LR
    subgraph SI[Service Instance]
      style SI fill:none,stroke-dasharray: 7 5
      B_Service["Service Logic"] <-->|Function Call| B_OPA["<img src='${logoPath}' width='30' height='30'/><br/>OPA"];
    end
    B_Policy["Policy & Data"] --> B_OPA;
  end

  subgraph AM[<b>Agent Model</b>]
    style AM fill:none,stroke:none;
    direction LR
    subgraph NP[Node/Pod]
      style NP fill:none,stroke-dasharray: 7 5
      direction LR
      subgraph "OPA Instance"
        A_OPA["<img src='${logoPath}' width='30' height='30' /><br/>OPA"];
      end
      subgraph "App Instance"
        A_Service["Service Logic"] <-->|HTTP Call| A_OPA
      end
    end
    A_Policy["Policy & Data"] --> A_OPA;
  end
`;

const HostLocalDiagram = () => <Mermaid value={diagram} />;

export default HostLocalDiagram;
