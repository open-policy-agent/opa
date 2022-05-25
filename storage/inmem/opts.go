package inmem

// An Opt modifies store at instantiation.
type Opt func(*store)

// OptRoundTripOnWrite sets whether incoming objects written to store are
// round-tripped through JSON to ensure they are serializable to JSON.
//
// Callers should disable use this if they can guarantee all objects passed to
// Write() are serializable to JSON. Failing to do so may result in undefined
// behavior, including panics.
//
// If setting to false consider deep-copying any objects passed to Write unless
// they can guarantee the objects will not be mutated after being written.
func OptRoundTripOnWrite(enabled bool) Opt {
	return func(s *store) {
		s.roundTripOnWrite = enabled
	}
}
