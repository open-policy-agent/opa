package timedateexample

import rego.v1

# Extract year, month, day from a nanosecond timestamp
# Useful for checking maintenance windows or date-based policies
date_parts := time.date(1720021249361794300)

year := date_parts[0]
month := date_parts[1]
day := date_parts[2]
