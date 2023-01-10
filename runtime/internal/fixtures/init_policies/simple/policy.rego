package init

import future.keywords.if
import future.keywords.contains

messages contains "old version" if {
  semver.compare(input.version, "0.49.0") < 0
}
