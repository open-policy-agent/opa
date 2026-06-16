package play

# A deployment pipeline defines the steps to bring a service up.
up_steps := ["provision", "configure", "deploy", "verify"]

# The teardown plan must run those same steps in reverse order.
expected_teardown := array.reverse(up_steps)

# Validate that the proposed teardown matches the reversed sequence.
teardown_valid if input.teardown == expected_teardown
