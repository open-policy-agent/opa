package pool

var keyToErrorMapPool = New(allocKeyToErrorMap, destroyKeyToErrorMap)

func allocKeyToErrorMap() interface{} {
	return make(map[string]error)
}

func KeyToErrorMap() *Pool[map[string]error] {
	return keyToErrorMapPool
}

func destroyKeyToErrorMap(m map[string]error) {
	for key := range m {
		delete(m, key)
	}
}
