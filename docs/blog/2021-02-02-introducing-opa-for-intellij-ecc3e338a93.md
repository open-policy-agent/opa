---
title: "Introducing OPA for IntelliJ"
authors: ["soothseer"]
date: 2021-02-02
slug: introducing-opa-for-intellij-ecc3e338a93
description: "OPA Plugin for IntelliJ IDEA brings Rego language support and an IDE experience to your OPA workflow!"
---

_[OPA Plugin](https://github.com/open-policy-agent/opa-idea-plugin) for IntelliJ IDEA brings Rego language support and an IDE experience to your OPA workflow!_

[IntelliJ IDEA](https://www.jetbrains.com/idea/) is one of the most popular IDEs for developers, offering built-in support for many programming languages such as Java, Kotlin, and Python. A new OPA plugin extends this support to policies written in the [Rego](https://www.openpolicyagent.org/docs/latest/policy-language/) query language, and it also works with [GoLand](https://www.jetbrains.com/go/) for Go developers. The plugin lets users write and evaluate policies directly inside the IDE.

![OPA Plugin on the JetBrains marketplace](/img/blog/introducing-opa-for-intellij-ecc3e338a93/banner.webp)

## Syntax Highlighting

The plugin includes support for rule heads, Rego keywords, function calls, strings, and comments.

![Rego Syntax Highlighting](/img/blog/introducing-opa-for-intellij-ecc3e338a93/2.png)

## OPA Actions Menu

From the OPA Actions Menu, users can run several key OPA actions on the workspace or the current open file:

![OPA Actions Menu](/img/blog/introducing-opa-for-intellij-ecc3e338a93/3.png)

- Install the OPA binary, if it isn't already installed (the plugin prompts for this when running other actions)
- **Format Document**: formats the open `.rego` file using `opa fmt`
- **Check Document**: checks the open `.rego` file using `opa check`
- **Test the workspace**: finds and runs all tests in the project (rules whose heads are prefixed with `test_`)
- **View test coverage** for the workspace
- **Display the trace** of selected code, using an `input.json` file found in the project root

## Run Configurations

In `.rego` files, every line containing a rule head or package includes a "Run Configuration" launch button in the gutter, allowing users to run `opa eval` or `opa test` directly.

![OPA Rules show Launch button next to them](/img/blog/introducing-opa-for-intellij-ecc3e338a93/4.png)

## Evaluate Rules

Running a configuration from a rule head line shows `opa eval` results for that package/rule, using the bundle directory and input file set in the configuration.

![OPA eval results panel for a rule](/img/blog/introducing-opa-for-intellij-ecc3e338a93/5.webp)

> Tip: you can use the shortcut `shift + F10` on Windows `^r` on Mac to re-run the last executed `Run Configuration`. It allows you to quickly check how changes affect your policies.

## Evaluate Packages

Running a configuration from a package line similarly shows `opa eval` results for the package, using the configured bundle directory and input file.

## Test Rules and Packages

Running a configuration from a `test_`-prefixed rule head runs `opa test`, with equivalent functionality available at the package level. Test results are displayed within the IDE.

![OPA test results displayed inside the IDE](/img/blog/introducing-opa-for-intellij-ecc3e338a93/6.webp)

## Coming Soon

### Eval and Partial Eval results for selected code

Future updates will let users highlight Rego code and view `eval` or `partial evaluation` results directly in the editor, using an `input.json` file in the project root. Results will appear as formatted JSON, and a profiling action will also be added to the menu.

## Who is it for?

The plugin aims to help newcomers to OPA/Rego with language support and quick access to core features, while giving experienced OPA developers a smoother in-IDE workflow.

## How To Contribute

The plugin is open source, hosted at [https://github.com/open-policy-agent/opa-idea-plugin](https://github.com/open-policy-agent/opa-idea-plugin). Contributors can file issues or pick up existing ones.

### Where To Start

The plugin is built with Kotlin and [Gradle](https://github.com/gradle/gradle). Newcomers to Kotlin are pointed to the official [Language Guide](https://kotlinlang.org/docs/reference/), and the bundled J2K Compiler ([tutorial link](https://kotlinlang.org/docs/tutorials/mixing-java-kotlin-intellij.html#converting-an-existing-java-file-to-kotlin-with-j2k)) can help convert Java boilerplate to Kotlin. The [IntelliJ Platform SDK DevGuide](https://jetbrains.org/intellij/sdk/docs/intro/welcome.html) is recommended for learning plugin development.

## Project Structure

A condensed overview of the source tree (full version at the [project's architecture page](https://github.com/open-policy-agent/opa-idea-plugin/blob/master/docs/devel/architecture.md)):

```
opa-idea-plugin/
├── gradle.build.kts
│   …
├── plugin # module to build/run/publish opa-ida-plugin plugin
   │   ...
   └── src/main/resources/META-INF/plugin.xml
           └── plugin.xml
├── idea # source code of features only available for IntelliJ IDEA
   │   ...
   └── src/main/kotlin/resources/META-INF
           └── idea-only.xml 
├── src # source code common to all IDEs
    ├── main
        ├── grammar
                └── Rego.bnf
        ├── kotlin/.../ideaplugin/
            ├── ide
                │   ...
                ├── actions
                └── extensions
            │   ...
            └── opa/tool
                └── OpaActions
        └── resources
            └── META-INF
                └── opa-core.xml
    └── test
        ├── kotlin/.../ideaplugin/
            │   ...
            ├── ide
            └── lang
        └── resources/.../ideaplugin/
```

## Note of Thanks

The author credits core contributors Vincent Gramer ([vgramer](https://github.com/vgramer/)), Frankie Cerkvenik ([frankiecerk](https://github.com/frankiecerk)), and Igor Rodzik ([irodzik](https://github.com/irodzik)) for seeding the project, with a full contributor list at [https://github.com/open-policy-agent/opa-idea-plugin/graphs/contributors](https://github.com/open-policy-agent/opa-idea-plugin/graphs/contributors). The plugin draws inspiration from the [IntelliJ Rust](https://github.com/intellij-rust/intellij-rust) project's reference implementation.
