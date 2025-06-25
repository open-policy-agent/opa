This next example shows doing the token signature verification, decoding, and content checks
all in one call using `io.jwt.decode_verify`. Note that this gives less flexibility in validating
the payload content as **all** claims defined in the JWT spec are verified with the provided
constraints.
