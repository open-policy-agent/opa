package play

# Whole GiB free; never over-count partial blocks.
bytes_per_gib := 1024 * 1024 * 1024

free_gib := floor(input.free_bytes / bytes_per_gib)
