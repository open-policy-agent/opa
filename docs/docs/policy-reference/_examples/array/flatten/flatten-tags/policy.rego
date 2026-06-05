package play

# Tags collected as [key, value] pairs, one pair per group.
grouped_tags := [["app", "web"], ["env", "prod"], ["team", "platform"]]

# Flatten the pairs into a single list so every tag can be checked in one pass.
flat_tags := array.flatten(grouped_tags)
