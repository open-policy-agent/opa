package pool

var stringSlicePool = New(allocStringSlice, destroyStringSlice)

func StringSlice() *Pool[*[]string] {
	return stringSlicePool
}

func allocStringSlice() interface{} {
	ret := make([]string, 0, 16)
	return &ret
}

func destroyStringSlice(s *[]string) {
	*s = (*s)[:0]
}
