---
sidebar_position: 9
sidebar_label: Language Server
---

<head>
  <title>Language Server | Regal</title>
</head>

# Language Server

In order to support Rego policy development in editors like
[VS Code](https://github.com/open-policy-agent/vscode-opa) or [Zed](https://github.com/StyraInc/zed-rego),
Regal provides an implementation of the
[Language Server Protocol](https://microsoft.github.io/language-server-protocol/) (LSP) for Rego.

This implementation allows the result of linting to be presented directly in your editor as you work on your policies,
and without having to call Regal from the command line. The language server however provides much more than just
linting!

:::tip
Check Regal's support for your editor on the
[editor support](https://www.openpolicyagent.org/projects/regal/editor-support)
page.
:::

## Features

The Regal language server currently supports the following LSP features:

### Diagnostics

Diagnostics are errors, warnings, and information messages that are shown in the editor as you type. Regal currently
uses diagnostics to present users with either parsing errors in case of syntax issues, and linter violations reported
by the Regal linter.

<img
  src={require('./assets/lsp/diagnostics.png').default}
  alt="Screenshot of diagnostics as displayed in Zed"/>

Future versions of Regal may include also [compilation errors](https://github.com/open-policy-agent/regal/issues/745)
as part of diagnostics messages.

### Hover

The hover feature means that moving the mouse over certain parts of the code will bring up a tooltip with documentation
for the code under the cursor. This is particularly useful for built-in functions, as it allows you to quickly look up
the meaning of the function, and the arguments it expects.

<img
  src={require('./assets/lsp/hover.png').default}
  alt="Screenshot of hover as displayed in VS Code"/>

The Regal language server currently supports hover for all built-in functions OPA provides.

### Go to definition

Go to definition allows references to rules and functions to be clicked on (while holding `ctrl/cmd`), and the editor
will navigate to the definition of the rule or function.

### Folding ranges

Regal provides folding ranges for any policy being edited. Folding ranges are areas of the code that can be collapsed
or expanded, which may be useful for hiding content that is not relevant to the current task.

<img
  src={require('./assets/lsp/folding.png').default}
  alt="Screenshot of folding ranges as displayed in Zed"/>

Regal supports folding ranges for blocks, imports and comments.

### Document and workspace symbols

Document and workspace symbols allow policy authors to quickly scan and navigate to symbols (like rules and functions)
anywhere in the document or workspace.

<img
  src={require('./assets/lsp/documentsymbols.png').default}
  alt="Screenshot showing search on workspace symbols in Zed"/>

VS Code additionally provides an "Outline" view, which is a nice visual representation of the symbols in the document.

<img
  src={require('./assets/lsp/documentsymbols2.png').default}
  alt="Screenshot showing outline view of document symbols in VS Code"/>

### Inlay hints

Inlay hints help developers quickly understand the meaning of the arguments passed passed to functions in the code,
by showing the name of the argument next to the value. Inlay hints can additionally be hovered for more information,
like the expected type of the argument.

<img
  src={require('./assets/lsp/inlay.png').default}
  alt="Screenshot showing inlay hints in VS Code"/>

Regal currently supports inlay hints for all built-in functions. Future versions may support inlay hints for
user-defined functions too.

### Formatting

By default, Regal uses the `opa fmt` formatter for formatting Rego. This is made available as a command in editors,
but also via a [code action](#code-actions) when unformatted files are encountered.

<img
  src={require('./assets/lsp/format.png').default}
  alt="Screenshot of diagnostics as displayed in Zed"/>

Two other formatters are also available — `opa fmt --rego-v1` and `regal fix`. See the docs on
[Fixing Violations](https://www.openpolicyagent.org/projects/regal/fixing) for more information about the `fix` command.
Which formatter to use can be set via the `formatter` configuration option, which can be passed to Regal via the client
(see the documentation for your client for how to do that).

### Code completions

Code completions, or suggestions, is likely one of the most useful features of the Regal language server. And best of
all, you don't need to do anything special for it to happen! Just write your policy as you normally would, and Regal
will provide suggestions for anything that could be relevant in the context that you're typing. This could for example
be suggestions for:

- Built-in functions
- Local variables
- Imported packages
- References from anywhere in the workspace
- And much more!

<img
  src={require('./assets/lsp/completion.png').default}
  alt="Screenshot of completion suggestions as displayed in Zed"/>

New completion providers are added continuously, so if you have a suggestion for
a new completion, please
[open an issue](https://github.com/open-policy-agent/regal/issues)!

### Code actions

Code actions are actions that appear in the editor when certain conditions are met. One example would be "quick fixes"
that may appear when a linter rule has been violated. Code actions can be triggered by clicking on the lightbulb icon
that appears on the line with a diagnostic message, or by pressing `ctrl/cmd + .` when the cursor is on the line.

<img
  src={require('./assets/lsp/codeaction.png').default}
  alt="Screenshot of code action displayed in Zed"/>

Regal currently provides **quick fix actions** for the following linter rules:

- [opa-fmt](https://www.openpolicyagent.org/projects/regal/rules/style/opa-fmt)
- [use-rego-v1](https://www.openpolicyagent.org/projects/regal/rules/imports/use-rego-v1)
- [use-assignment-operator](https://www.openpolicyagent.org/projects/regal/rules/style/use-assignment-operator)
- [no-whitespace-comment](https://www.openpolicyagent.org/projects/regal/rules/style/no-whitespace-comment)
- [directory-package-mismatch](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/directory-package-mismatch)

Regal also provides **source actions** — actions that apply to a whole file and aren't triggered by linter issues:

- **Explore compiler stages for policy** — Opens a browser window with an embedded version of the
  [opa-explorer](https://github.com/srenatus/opa-explorer), where advanced users can explore the different stages
  of the Rego compiler's output for a given policy.

### Code lenses (Evaluation)

The code lens feature provides language servers a way to add actionable commands just next to the code that the action
belongs to. Regal provides code lenses for doing **evaluation** of any package or rule directly in the editor. This
allows for an extremely fast feedback loop, where you can see the result of writing of modifying rules directly as you
work with them, and without having to launch external commands or terminals. In any editor that supports code lenses,
simply press `Evaluate` on top of a package or rule declaration to have it evaluated. The result is displayed on the
same line.

<img
  src={require('./assets/lsp/evalcodelens.png').default}
  alt="Screenshot of evaluation performed via code lens"/>

Once evaluation has completed, the result is also pretty-printed in a tooltip when hovering the rule. This is
particularly useful when the result contains more data than can fit on a single line!

Note that when evaluating incrementally defined rules, the result reflects evaluation of the whole **document**,
not a single rule definition. To make this clear, the result will be displayed next to each definition of the
same rule.

In addition to showing the result of evaluation, the "Evaluate" code lens will also display the output of any
`print` calls made in rule bodies. This can be really helpful when trying to figure out _why_ the rule evaluated
the way it did, or where rule evaluation failed.

<img
  src={require('./assets/lsp/evalcodelensprint.png').default}
  alt="Screenshot of evaluation with print call performed via code lens"/>

Policy evaluation often depends on **input**. This can be provided via an `input.json` or `input.yaml` file which
Regal will search for first in the same directory as the policy file evaluated. If not found there, Regal will proceed
to search each parent directory up until the workspace root directory. It is recommended to add `input.json/yaml` to
your `.gitignore` file so that you can work freely with evaluation in any directory without having your input
accidentally committed.

#### Editor support

The Evaluation code lens is supported in any language server client that supports the running of code lenses. The
evaluation result is saved to `output.json` in the default case.

The displaying of evaluation results in the current file or buffer is currently only supported in the
[OPA VS Code extension](https://github.com/open-policy-agent/vscode-opa) and for Neovim users in
[nvim-dap-rego](https://github.com/rinx/nvim-dap-rego/).

### Code lenses (Debugging)

Regal also implements the [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/). This allows
users to trigger debugging sessions for their policies by invoking a code lens on a rule. For more information, see the
[Debug Adapter](https://www.openpolicyagent.org/projects/regal/debug-adapter) page.

#### Editor support

While the code lens feature is part of the LSP specification, the action that is triggered by a code lens
isn't necessarily part of the standard. The language server protocol does not provide a native method for requesting
evaluation, so Regal will handle that on its own, and differently depending on what the client supports.

- Currently, only the [OPA VS Code extension](https://github.com/open-policy-agent/vscode-opa) and
  [nvim-dap-rego](https://github.com/rinx/nvim-dap-rego/) is capable of handling
  the request to display evaluation results on the same line as the package or rule evaluated.
- [Neovim](https://neovim.io/) does not support the requests natively, but
  [nvim-dap-rego](https://github.com/rinx/nvim-dap-rego/) provides handlers to support them.
  Please follow [the instructions](https://github.com/rinx/nvim-dap-rego/blob/main/README.md#lsp-handlers) in
  nvim-dap-rego README.
- [Zed](https://github.com/StyraInc/zed-rego) does not support the code lens feature at all at this point in time. As
  soon as it does, Regal will provide them.
- Displaying the result of evaluation requires customized code in the client. Currently only VS Code and Neovim
  has the required modifications to handle this, and is thus the only editor to currently support "inline display"
  of the result.
  For other editors that support the code lens feature, Regal will instead write the result of evaluation to an
  `output.json` file.

### Selection ranges

<img
  src={require('./assets/lsp/selectionranges.gif').default}
  alt="Animation showing expanding selection range to parent AST nodes"/>

Selection ranges allow expanding and shrinking of selections in the editor based on the syntactic structure of the code.
Say for example that you have the following code:

```rego
package example

my_rule if {
    multi.part.reference == true
}
```

With the cursor somewhere on the `part` term, expanding the selection range would first select the `part` term, then
expand further to select the whole `multi.part.reference` reference, then the whole equality expression, then the whole
rule body, and so on. This can be extremely efficient when selecting code for copying, cutting, or replacing. Note also
_ranges_ in plural here — as this feature supports multiple cursors/selections at once.

#### Editor support

##### VS Code

- For best results, set `editor.smartSelect.selectLeadingAndTrailingWhitespace` and `editor.smartSelect.selectSubwords`
  to `false` in your VS Code settings, as this will let Regal control the selection ranges fully
- Default keybindings for selection ranges are:
  - Grow selection: `Shift` + `Alt` + `Right Arrow` (`Ctrl` + `Shift` + `Right Arrow` on Mac)
  - Shrink selection: `Shift + Alt + Left Arrow` (`Ctrl` + `Shift` + `Left Arrow` on Mac)
- See the configuration of binding for the `editor.action.smartSelect.grow` and `editor.action.smartSelect.shrink`
  commands, should you want to change them

### Linked editing ranges

Linked editing ranges allow renaming of local symbols in multiple places at once. The most well-known example of this is
in HTML/XML editing, where renaming a tag will update both the opening and closing tag at the same time. This feature is
however of limited value in most other languages, and therefore typically disabled by default in editors. While Regal's
language server contains experimental code that links edits of function arguments to references of those variables in
the function head or body, we'd rather implement the rename feature from the LSP specification for this purpose. For
that reason, the linked editing ranges feature is currently disabled by default. Set the `REGAL_EXPERIMENTAL`
environment variable to `true` if you want to try it out, but remember that you may also have to enable linked editing
in your editor.

If you have any suggestions for how linked editing ranges could be useful in Rego, please
[open an issue](https://github.com/open-policy-agent/regal/issues/new) to let us know!

### Signature Help

Signature help is a feature that shows the names and types of variables as the
user types a call to a function.

<img
  src={require('./assets/lsp/signaturehelp.gif').default}
  alt="Animation showing suggestions for function arguments"/>

### Document Highlights

Document highlights are regions of the current file that deserve additional
attention. We use document highlights in Regal to show usages of a variable
within a function - more use cases coming soon!

<img
  src={require('./assets/lsp/documenthighlights.gif').default}
  alt="Animation showing highlighting of values in a function"/>

### Document Links

Document links are used to make regular text ranges within a file appear as a
clickable link in clients. We use these links to make ignore directives
clickable.

<img
  src={require('./assets/lsp/documentlinks.gif').default}
  alt="Animation showing a document link in action"/>

## Unsupported features

See the
[open issues](https://github.com/open-policy-agent/regal/issues?q=is%3Aissue+is%3Aopen+label%3A%22language+server+protocol%22)
with the `language server protocol` label for a list of features that are not yet supported by the Regal language
server, but that are planned for the future. If you have suggestions for anything else, please create a new issue!

Also note that not all clients (i.e. editors) may support all features of a language server! See the
[editor support](https://www.openpolicyagent.org/projects/regal/editor-support) page for information about Regal support
in different editors.
