package play

min_replicas := 2
max_replicas := 10

# Raise to HA floor, then cap for cost limits.
raised := max([min_replicas, input.requested_replicas])
effective_replicas := min([max_replicas, raised])
