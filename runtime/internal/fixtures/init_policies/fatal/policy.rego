package init

import future.keywords.if
import future.keywords.contains

fatals contains "version is insecure and not safe for production use, please upgrade" if {
  semver.compare(input.version, "0.10.0") < 0
}
