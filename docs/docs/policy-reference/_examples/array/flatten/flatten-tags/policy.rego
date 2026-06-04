package play

# array.flatten merges the sub-arrays of an array into a single array,
# removing one level of nesting.
grouped_tags := [["app", "web"], ["env", "prod"], ["team", "platform"]]

flat_tags := array.flatten(grouped_tags)
