package timeweekdayexample

import rego.v1

# Get the day of the week from a timestamp
# Useful for enforcing deployment freeze on weekends
day := time.weekday(1720021249361794300)

# Check if it is a workday (not Saturday or Sunday)
is_workday if {
    day != "Saturday"
    day != "Sunday"
}
