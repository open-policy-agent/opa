package init

import future.keywords.if
import future.keywords.contains

dismiss_env_var := "OPA_VERSION_DEPRECATED"

errors contains {
  "message": sprintf("version is deprecated, please upgrade or set %s to start this deprecated OPA", [dismiss_env_var]),
  "var": dismiss_env_var,
  "dismissed": object.get(input.env, dismiss_env_var, "") != ""
} if {
  semver.compare(input.version, "0.10.0") < 0
}
