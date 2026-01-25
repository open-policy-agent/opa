package play

import rego.v1

# Parse decimal SI units (base 1000)
kilobytes := units.parse("10K") # Result: 10000

megabytes := units.parse("200M") # Result: 200000000

gigabytes := units.parse("100G") # Result: 100000000000

# Parse milli-units (base 0.001)
milliunits := units.parse("100m") # Result: 0.1

# Parse binary units (base 1024)
kibibytes := units.parse("10Ki") # Result: 10240

mebibytes := units.parse("100Mi") # Result: 104857600

gibibytes := units.parse("5Gi") # Result: 5368709120
