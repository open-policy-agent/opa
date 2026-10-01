package play

# Never under-provision: require whole CPU cores.
cpu_cores_required := ceil(input.cpu_request)
