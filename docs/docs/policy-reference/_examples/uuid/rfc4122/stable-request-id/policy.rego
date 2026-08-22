package play

# Same key returns the same UUID within this evaluation:
req_id1 := uuid.rfc4122("req-1")
req_id2 := uuid.rfc4122("req-1")
same_key_match := req_id1 == req_id2

# Different key returns a different UUID:
other_id := uuid.rfc4122("req-2")
diff_key_match := req_id1 == other_id
