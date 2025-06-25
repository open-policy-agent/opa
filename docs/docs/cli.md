---
title: CLI
---

The commands exposed in the `opa` executable are listed here in alphabetical
order.

:::tip
Note that command line arguments may either be provided as traditional flags, or
as environment variables. The expected format of environment variables used for
this purpose follows the pattern `OPA_<COMMAND>_<FLAG>` where COMMAND is the
command name in uppercase (like EVAL) and FLAG is the flag name in uppercase
(like STRICT), i.e. `OPA_EVAL_STRICT` would be equivalent to passing the
--strict flag to the eval command.
:::

import commands from "@generated/cli-data/default/cli.json";
import CommandList from "@site/src/components/CommandList";

<CommandList commands={commands} />

export const toc = commands.map(command => ({ value: command.id, id: command.id, level: 2 }));
