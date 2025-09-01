---
sidebar_position: 10
---


# Debug Adapter

In addition to being a [language server](https://openpolicyagent.org/projects/regal/language-server),
Regal can act as a
[Debug Adapter](https://microsoft.github.io/debug-adapter-protocol/).
A Debug Adapter is a program that can communicate with a debugger client,
such as Visual Studio Code's debugger, to provide debugging capabilities
for a language.

<img
  src={require('./assets/dap/animation.gif').default}
  alt="Animation showing the a debugging session in VS Code"/>
_A debugging session in VS Code_

:::info
In order to use the Debug Adapter, you must be using
[Regal v0.27.0](https://github.com/open-policy-agent/regal/releases/v0.27.0) or greater,
as well as a compatible client. See [Editor Support](https://openpolicyagent.org/projects/regal/editor-support) for
more details.
:::

## Getting Started

See the documentation in the Editor Support page for supported clients:

* [Visual Studio Code](https://openpolicyagent.org/projects/regal/editor-support#visual-studio-code)
* [Neovim](https://openpolicyagent.org/projects/regal/editor-support#neovim)

## Features

The Regal Debug Adapter currently supports the following features:

### Breakpoints

Breakpoints allow you to continue execution of a policy until a given point.
This can be helpful for:

* Inspection of variables at a given point in time
* Seeing how many times a given block of Rego code is executed, if at all
* Avoiding the need to step through code as it's run line by line

<img
  src={require('./assets/dap/breakpoint.png').default}
  alt="Screenshot of a breakpoint in VS Code"/>

### Variable Inspection

Either at a breakpoint or while stepping through code, you can inspect the
local variables in scope as well as the contents of the global `input` and
`data` documents.

`input` will be loaded from `input.json` in the workspace if it exists.

<img
  src={require('./assets/dap/variables.png').default}
  alt="Variables being inspected during execution in VS Code"/>

### Print Statements

Print statements are also supported, these are shown in the debug console:

<img
  src={require('./assets/dap/print.png').default}
  alt="Print statements shown in the debug output console"/>
