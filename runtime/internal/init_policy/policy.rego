package init

import future.keywords.if
import future.keywords.contains

dismiss_env_var := "OPA_DISMISS"

messages contains "things" if true

errors contains {
  "message": "dismissable error",
  "var": dismiss_env_var,
  "dismissed": object.get(input.env, dismiss_env_var, "") != ""
}

fatals contains "failed"
