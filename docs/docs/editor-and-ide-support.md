---
title: Editor and IDE Support
---

OPA can be integrated into editors and IDEs to provide features like syntax highlighting, query
evaluation, policy coverage, and more.

| Editor             | Link                                                                                                                                                                                                                                                                                                                                         | Note                                       |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| Visual Studio Code | [marketplace.visualstudio.com/items?itemName=tsandall.opa](https://marketplace.visualstudio.com/items?itemName=tsandall.opa)                                                                                                                                                                                                                 | Supports Language Server and Debug Adapter |
| Neovim             | Syntax highlighting [tree-sitter-rego](https://github.com/FallenAngel97/tree-sitter-rego), Language server [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig/blob/master/doc/server_configurations.md#regal), Debugger [nvim-dap](https://github.com/mfussenegger/nvim-dap) + [nvim-dap-rego](https://github.com/rinx/nvim-dap-rego) | Supports Language Server and Debug Adapter |
| Zed                | [github.com/StyraOSS/zed-rego](https://github.com/StyraOSS/zed-rego)                                                                                                                                                                                                                                                                         | Supports Language Server                   |
| IntelliJ IDEA      | [github.com/open-policy-agent/opa-idea-plugin](https://github.com/open-policy-agent/opa-idea-plugin)                                                                                                                                                                                                                                         |                                            |
| Vim                | [github.com/tsandall/vim-rego](https://github.com/tsandall/vim-rego)                                                                                                                                                                                                                                                                         |                                            |
| Emacs              | [github.com/psibi/rego-mode](https://github.com/psibi/rego-mode)                                                                                                                                                                                                                                                                             |                                            |
| Nano               | [github.com/scopatz/nanorc](https://github.com/scopatz/nanorc)                                                                                                                                                                                                                                                                               |                                            |
| Sublime Text       | [github.com/open-policy-agent/opa/tree/main/misc/syntax/sublime](https://github.com/open-policy-agent/opa/tree/main/misc/syntax/sublime)                                                                                                                                                                                                     |                                            |
| TextMate           | [github.com/open-policy-agent/opa/tree/main/misc/syntax/textmate](https://github.com/open-policy-agent/opa/tree/main/misc/syntax/textmate)                                                                                                                                                                                                   |                                            |

:::info
**Your editor missing? Built a Rego integration for your editor?** Send a
message on [Slack](https://slack.openpolicyagent.org)
The [Ecosystem page](/ecosystem/) is also a great place to
showcase your project. See
[these instructions](./contrib-docs#opa-ecosystem-additions)
to get it listed.
:::

## Rego Playground

The Rego Playground provides a great editor to get started with OPA and share
policies. Try it out at
[play.openpolicyagent.org](https://play.openpolicyagent.org/).
