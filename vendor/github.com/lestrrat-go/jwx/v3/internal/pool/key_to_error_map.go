package pool

var keyToErrorMapPool = New[map[string]error](allocKeyToErrorMap, freeKeyToErrorMap)

func allocKeyToErrorMap() map[string]error {
	return make(map[string]error)
}

func freeKeyToErrorMap(m map[string]error) map[string]error {
	for k := range m {
		delete(m, k) // Clear the map
	}
	return m
}

// KeyToErrorMap returns a pool of map[string]error instances.
func KeyToErrorMap() *Pool[map[string]error] {
	return keyToErrorMapPool
}
