---
title: VS Code Extension
subtitle: OPA Integration for the VS Code editor
labels:
  category: tooling
  layer: editor
code:
- https://github.com/open-policy-agent/vscode-opa
videos:
- https://www.youtube.com/watch?v=BpMttxuPv6Y
tutorials:
- https://docs.styra.com/regal/editor-support#visual-studio-code
software:
- editors
inventors:
- styra
docs_features:
  editors:
    note: |
      This is the official VS Code extension for Rego and OPA. The
      extension brings a number of features that make writing Rego easier
      such as syntax highlighting, running of OPA tests as well as
      completions, linting and more from the
      [Regal Language Server](/integrations/regal/).
  debugging-rego:
    note: |
      The extension provides support for debugging Rego policies using the
      native VS Code debugging interface. This is based on Regal's Debug
      Adapter, see the
      [VS Code documentation](https://docs.styra.com/regal/editor-support#visual-studio-code)
      to get started.
---

The [vscode-opa extension](https://marketplace.visualstudio.com/items?itemName=tsandall.opa)
is a Visual Studio Code extension that provides support for the Rego language
and OPA functionality. The extension includes syntax highlighting, and first
class support for the Regal
[Language Server](https://docs.styra.com/regal/language-server) and
[Debug Adapter](https://docs.styra.com/regal/debug-adapter).
