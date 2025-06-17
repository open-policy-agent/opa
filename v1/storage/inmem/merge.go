package inmem

import (
	"maps"
	"sync"
)

// InterfaceMaps returns the result of merging a and b. If a and b cannot be
// merged because of conflicting key-value pairs, ok is false.
func InterfaceMaps(a map[string]any, b map[string]any) (map[string]any, bool) {

	if a == nil {
		return b, true
	}

	if hasConflicts(a, b) {
		return nil, false
	}

	return merge(a, b), true
}

func merge(a, b map[string]any) map[string]any {
	// Optimized merge strategy
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	// If a is much smaller than b, create a new map
	if len(a)*3 < len(b) {
		result := make(map[string]any, len(a)+len(b))
		maps.Copy(result, a)
		a = result
	}

	for k := range b {
		add := b[k]
		exist, ok := a[k]
		if !ok {
			a[k] = add
			continue
		}

		existObj, existOk := exist.(map[string]any)
		addObj, addOk := add.(map[string]any)

		if existOk && addOk {
			a[k] = merge(existObj, addObj)
		} else {
			a[k] = add
		}
	}

	return a
}

func hasConflicts(a, b map[string]any) bool {
	for k := range b {

		add := b[k]
		exist, ok := a[k]
		if !ok {
			continue
		}

		existObj, existOk := exist.(map[string]any)
		addObj, addOk := add.(map[string]any)
		if !existOk || !addOk {
			return true
		}

		if hasConflicts(existObj, addObj) {
			return true
		}
	}
	return false
}

// Optimized DeepCopy function with object pooling
var (
	slicePool = sync.Pool{
		New: func() any {
			// Store pointers to slices
			s := make([]any, 0, 8)
			return &s
		},
	}
	mapPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 8)
		},
	}
)

func DeepCopy(val any) any {
	switch v := val.(type) {
	case []any:
		if len(v) == 0 {
			return v
		}
		// Попытка получить временный срез из пула
		poolSlice := slicePool.Get().(*[]any)
		// Если емкости недостаточно – создаём новый срез
		if cap(*poolSlice) < len(v) {
			*poolSlice = make([]any, len(v))
		} else {
			*poolSlice = (*poolSlice)[:len(v)]
		}
		for i, item := range v {
			(*poolSlice)[i] = DeepCopy(item)
		}
		// Копируем результат в новый срез именно требуемой длины
		res := make([]any, len(v))
		copy(res, *poolSlice)
		slicePool.Put(poolSlice)
		return res
	case map[string]any:
		if len(v) == 0 {
			return v
		}
		// Попытка получить карту из пула
		poolMap := mapPool.Get().(map[string]any)
		// Очищаем карту перед использованием
		for k := range poolMap {
			delete(poolMap, k)
		}
		for key, item := range v {
			poolMap[key] = DeepCopy(item)
		}
		// Копируем результат в новую карту (будет иметь точную вместимость)
		res := make(map[string]any, len(poolMap))
		maps.Copy(res, poolMap)
		mapPool.Put(poolMap)
		return res
	default:
		return v
	}
}

func Map(val map[string]any) map[string]any {
	if len(val) == 0 {
		return val
	}
	// Получаем карту из пула
	tmp := mapPool.Get().(map[string]any)
	// Очищаем карту от предыдущих значений
	for k := range tmp {
		delete(tmp, k)
	}
	for k, v := range val {
		tmp[k] = DeepCopy(v)
	}
	// Создаём точную копию и возвращаем временную карту обратно в пул
	res := make(map[string]any, len(tmp))
	maps.Copy(res, tmp)
	mapPool.Put(tmp)
	return res
}
