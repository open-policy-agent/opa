package pool

var mapPool = New(allocMap, destroyMap)

func Map() *Pool[*map[string]interface{}] {
	return mapPool
}

func allocMap() interface{} {
	m := make(map[string]interface{})
	return &m
}

func destroyMap(m *map[string]interface{}) {
	if len(*m) > 16 {
		*m = make(map[string]interface{})
		return
	}

	for key := range *m {
		delete(*m, key)
	}
}
