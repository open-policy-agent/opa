#include <unordered_map>

#include "malloc.h"
#include "regex.h"
#include "test.h"


WASM_EXPORT(test_regex_cache)
extern "C"
void test_regex_cache(void)
{
	// Reset the cache
	opa_builtin_cache_set(0, NULL);

	for (int i = 0; i < 100; i++)
	{
		char pattern[20];
		snprintf(pattern, sizeof(pattern), "foo%d.*", i);
		opa_regex_match(opa_string_terminated(pattern), opa_string_terminated("foobar"));
	}

	std::unordered_map<void*, void*>* c = static_cast<std::unordered_map<void*, void*>*>(opa_builtin_cache_get(0));

	test("regex cache size", c->size() == 100);

	opa_regex_match(opa_string_terminated("bar.*"), opa_string_terminated("barbaz"));

	test("regex cache size doesn't surpass max", c->size() == 100);
}