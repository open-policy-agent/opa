#include <ctype.h>

#include "aggregates.h"
#include "arithmetic.h"
#include "array.h"
#include "bits-builtins.h"
#include "cidr.h"
#include "conversions.h"
#include "encoding.h"
#include "glob.h"
#include "graphs.h"
#include "json.h"
#include "malloc.h"
#include "memoize.h"
#include "mpd.h"
#include "numbers.h"
#include "object.h"
#include "regex.h"
#include "set.h"
#include "str.h"
#include "strings.h"
#include "test.h"
#include "types.h"

// NOTE(sr): we've removed the float number representation, so this helper
// is to make our tests less annoying:
#define opa_number_float(f) opa_number_ref(#f, sizeof(#f))

void reset_heap(void)
{
    // This will leak memory!!
    // TODO: How should we safely reset it if we don't know the original starting ptr?
    opa_heap_ptr_set(opa_heap_top_get());
}

WASM_EXPORT(test_opa_malloc)
void test_opa_malloc(void)
{
    opa_malloc_init_test();

    reset_heap();

    // NOTE(tsandall): These numbers are not particularly important. They're
    // sized to cause opa_malloc to call grow.memory. The tester initializes
    // memory with 2 pages so we allocate ~4 pages of memory here.
    const int N = 256;
    const int S = 1024;

    for(int i = 0; i < N; i++)
    {
        char *buf = opa_malloc(S);

        for(int x = 0; x < S; x++)
        {
            buf[x] = x % 255;
        }
    }
}

WASM_EXPORT(test_opa_malloc_min_size)
void test_opa_malloc_min_size(void)
{
    reset_heap();

    // Ensure that allocations less than the minimum size
    // are creating blocks large enough to be re-used by
    // the minimum size.
    void *too_small = opa_malloc(2);
    test("allocated min block", too_small != NULL);

    void *barrier = opa_malloc(0);

    opa_free(too_small);

    test("new free block", opa_heap_free_blocks() == 1);

    void *min_sized = opa_malloc(4);
    test("reused block", opa_heap_free_blocks() == 0);
    opa_free(min_sized);
    opa_free(barrier);
}

WASM_EXPORT(test_opa_malloc_split_threshold_small_block)
void test_opa_malloc_split_threshold_small_block(void)
{
    reset_heap();

    // Ensure that free blocks larger than the requested
    // allocation, but too small to leave a sufficiently
    // sized remainder, are left intact.
    size_t heap_block_size = 12;
    void *too_small = opa_malloc(2 * 128 + heap_block_size - 1);
    test("allocated too_small block", too_small != NULL);

    void *barrier = opa_malloc(256);

    opa_free(too_small);

    test("new small free block", opa_heap_free_blocks() == 1);

    // Expect the smaller allocation to use the bigger block
    // without splitting.
    void *new = opa_malloc(128);
    test("unable to split block", opa_heap_free_blocks() == 0);
    opa_free(new);
}

WASM_EXPORT(test_opa_malloc_split_threshold_big_block)
void test_opa_malloc_split_threshold_big_block(void)
{
    reset_heap();

    // Ensure that free blocks large enough to be split are split up
    // until they are too small: have space almost to allocate three
    // separate 128 blocks.
    size_t heap_block_size = 12;
    void *splittable = opa_malloc(3 * 128 + 2 * heap_block_size - 1);
    test("allocated splittable block", splittable != NULL);

    void *barrier = opa_malloc(128);

    opa_free(splittable);
    test("new large free block", opa_heap_free_blocks() == 1);

    // Expect to be able to get multiple blocks out of the new free one without
    // new allocations.
    unsigned int high = opa_heap_ptr_get();

    void *split1 = opa_malloc(128);
    void *split2 = opa_malloc(128);  // Too big to split remaining bytes, should take oversized block.

    test("heap ptr", high == opa_heap_ptr_get());
    test("remaining free blocks", opa_heap_free_blocks() == 0);

    opa_free(split1);
    opa_free(split2);
    opa_free(barrier);
}

WASM_EXPORT(test_opa_free)
void test_opa_free(void)
{
    reset_heap();


    // check the heap shrinks with a single malloc and free.

    size_t blocks = opa_heap_free_blocks();
    unsigned int base = opa_heap_ptr_get();
    opa_free(opa_malloc(0));

    test("heap ptr", base == opa_heap_ptr_get());
    test("free blocks", blocks == 0 && opa_heap_free_blocks() == 0);

    // check the double malloc, followed with frees in identical order
    // results in eventual heap shrinking.

    void *p1 = opa_malloc(0);
    void *p2 = opa_malloc(0);
    unsigned int high = opa_heap_ptr_get();
    test("free blocks", opa_heap_free_blocks() == 0);

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 1);
    test("heap ptr", high == opa_heap_ptr_get());

    opa_free(p2);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", base == opa_heap_ptr_get());

    // check the double malloc, followed with frees in reverse order
    // results in gradual heap shrinking.

    p1 = opa_malloc(0);
    p2 = opa_malloc(0);
    high = opa_heap_ptr_get();
    test("free blocks", opa_heap_free_blocks() == 0);

    opa_free(p2);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", high > opa_heap_ptr_get());

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", base == opa_heap_ptr_get());

    // check the free re-use (without splitting).

    p1 = opa_malloc(1);
    p2 = opa_malloc(1);
    high = opa_heap_ptr_get();

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 1);

    p1 = opa_malloc(1);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", high == opa_heap_ptr_get());

    opa_free(p2);
    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", base == opa_heap_ptr_get());

    // check the free re-use (with splitting).

    p1 = opa_malloc(512);
    p2 = opa_malloc(512);
    high = opa_heap_ptr_get();

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 1);

    p1 = opa_malloc(128);
    test("free blocks", opa_heap_free_blocks() == 1);
    test("heap ptr", high == opa_heap_ptr_get());

    opa_free(p2);
    test("free blocks", opa_heap_free_blocks() == 0);

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", base == opa_heap_ptr_get());
}

WASM_EXPORT(test_opa_memoize)
void test_opa_memoize(void)
{
    opa_memoize_init();

    opa_memoize_insert(100, opa_number_int(1));
    opa_memoize_insert(200, opa_number_int(2));
    opa_value *a = opa_memoize_get(100);
    opa_value *b = opa_memoize_get(200);
    opa_memoize_push();
    opa_value *c = opa_memoize_get(100);
    opa_value *d = opa_memoize_get(200);
    opa_memoize_insert(100, opa_number_int(3));
    opa_memoize_pop();
    opa_value *e = opa_memoize_get(100);

    opa_value *exp_a = opa_number_int(1);
    opa_value *exp_b = opa_number_int(2);
    opa_value *exp_c = NULL;
    opa_value *exp_d = NULL;
    opa_value *exp_e = opa_number_int(1);

    test("insert-a", opa_value_compare(a, exp_a) == 0);
    test("insert-b", opa_value_compare(b, exp_b) == 0);
    test("get-a-after-push", opa_value_compare(c, exp_c) == 0);
    test("get-b-after-push", opa_value_compare(d, exp_d) == 0);
    test("get-a-after-pop", opa_value_compare(e, exp_e) == 0);
}

// NOTE(sr): These tests are run in order. If they weren't, every test that
// depends on mpd's state being initialized would have to call `opa_mpd_init`
// first. When the Wasm module is used, the `Start` function (`_initialize`,
// emitted from the Wasm compiler) takes care of that.
WASM_EXPORT(test_opa_mpd)
void test_opa_mpd(void)
{
    // NOTE(sr): This call also initializes mpd_one, which is used under the
    // hood for `qadd_one`.
    opa_mpd_init();
    opa_value *zero = opa_number_int(0);
    opa_value *two = opa_bf_to_number(qadd_one(qadd_one(opa_number_to_bf(zero))));
    test("0+1+1 is 2", opa_value_compare(opa_number_int(2), two) == 0);
}

WASM_EXPORT(test_opa_strlen)
void test_opa_strlen(void)
{
    test("empty", opa_strlen("") == 0);
    test("non-empty", opa_strlen("1234") == 4);
}

WASM_EXPORT(test_opa_strncmp)
void test_opa_strncmp(void)
{
    test("empty", opa_strncmp("", "", 0) == 0);
    test("equal", opa_strncmp("1234", "1234", 4) == 0);
    test("less than", opa_strncmp("1234", "1243", 4) < 0);
    test("greater than", opa_strncmp("1243", "1234", 4) > 0);
}

WASM_EXPORT(test_opa_strcmp)
void test_opa_strcmp(void)
{
    test("empty", opa_strcmp("", "") == 0);
    test("equal", opa_strcmp("abcd", "abcd") == 0);
    test("less than", opa_strcmp("1234", "1243") < 0);
    test("greater than", opa_strcmp("1243", "1234") > 0);
    test("shorter", opa_strcmp("123", "1234") < 0);
    test("longer", opa_strcmp("1234", "123") > 0);
}

WASM_EXPORT(test_opa_itoa)
void test_opa_itoa(void)
{
    char buf[sizeof(long long)*8+1];

    test("itoa", opa_strcmp(opa_itoa(0, buf, 10), "0") == 0);
    test("itoa", opa_strcmp(opa_itoa(-128, buf, 10), "-128") == 0);
    test("itoa", opa_strcmp(opa_itoa(127, buf, 10), "127") == 0);
    test("itoa", opa_strcmp(opa_itoa(0x7FFFFFFFFFFFFFFF, buf, 10), "9223372036854775807") == 0);
    test("itoa", opa_strcmp(opa_itoa(0x8000000000000001, buf, 10), "-9223372036854775807") == 0);
    test("itoa", opa_strcmp(opa_itoa(0xFFFFFFFFFFFFFFFF, buf, 10), "-1") == 0);

    test("itoa/base2", opa_strcmp(opa_itoa(0, buf, 2), "0") == 0);
    test("itoa/base2", opa_strcmp(opa_itoa(-128, buf, 2), "-10000000") == 0);
    test("itoa/base2", opa_strcmp(opa_itoa(127, buf, 2), "1111111") == 0);
    test("itoa/base2", opa_strcmp(opa_itoa(0x7FFFFFFFFFFFFFFF, buf, 2), "111111111111111111111111111111111111111111111111111111111111111") == 0);
    test("itoa/base2", opa_strcmp(opa_itoa(0x8000000000000001, buf, 2), "-111111111111111111111111111111111111111111111111111111111111111") == 0);
    test("itoa/base2", opa_strcmp(opa_itoa(0xFFFFFFFFFFFFFFFF, buf, 2), "-1") == 0);

    test("itoa/base16", opa_strcmp(opa_itoa(0, buf, 16), "0") == 0);
    test("itoa/base16", opa_strcmp(opa_itoa(-128, buf, 16), "-80") == 0);
    test("itoa/base16", opa_strcmp(opa_itoa(127, buf, 16), "7f") == 0);
    test("itoa/base16", opa_strcmp(opa_itoa(0x7FFFFFFFFFFFFFFF, buf,16), "7fffffffffffffff") == 0);
    test("itoa/base16", opa_strcmp(opa_itoa(0x8000000000000001, buf, 16), "-7fffffffffffffff") == 0);
    test("itoa/base16", opa_strcmp(opa_itoa(0xFFFFFFFFFFFFFFFF, buf, 16), "-1") == 0);
}


int crunch_opa_atoi64(const char *str, long long exp, int exp_rc)
{
    long long result;
    int rc;

    if ((rc = opa_atoi64(str, opa_strlen(str), &result)) != exp_rc)
    {
        return 0;
    }

    return exp_rc != 0 || result == exp;
}

WASM_EXPORT(test_opa_atoi64)
void test_opa_atoi64(void)
{
    test("integer", crunch_opa_atoi64("127", 127, 0));
    test("negative integer", crunch_opa_atoi64("-128", -128, 0));
    test("non integer", crunch_opa_atoi64("-128.3", 0, -2));
    test("empty", crunch_opa_atoi64("", 0, -1));
}

int crunch_opa_atof64(const char *str, double exp, int exp_rc)
{
    double result;
    int rc;

    if ((rc = opa_atof64(str, opa_strlen(str), &result)) != exp_rc)
    {
        return 0;
    }

    return exp_rc != 0 || result == exp;
}

WASM_EXPORT(test_opa_atof64)
void test_opa_atof64(void)
{
    test("empty", crunch_opa_atof64("", 0, -1));
    test("bad integer", crunch_opa_atof64("1234-6", 0, -2));
    test("bad fraction", crunch_opa_atof64("1234.5-6", 0, -2));
    test("bad exponent", crunch_opa_atof64("1234.5e6-", 0, -2));
    test("bad exponent", crunch_opa_atof64("12345e6-", 0, -2));
    test("integer", crunch_opa_atof64("127", 127, 0));
    test("negative integer", crunch_opa_atof64("-128", -128, 0));
    test("fraction", crunch_opa_atof64("16.7", 16.7, 0));
    test("exponent", crunch_opa_atof64("6e7", 6e7, 0));
}

WASM_EXPORT(test_memchr)
void test_memchr(void)
{
    char s[] = { 1, 2, 2, 3 };

    test("memchr", memchr(s, 2, 1) == NULL);
    test("memchr", memchr(s, 2, sizeof(s)) == &s[1]);
    test("memchr", memchr(s, 4, sizeof(s)) == NULL);
}

WASM_EXPORT(test_memcmp)
void test_memcmp(void)
{
    char a[] = { 1, 2, 3, 4 }, b[] = { 1, 2, 3, 3 };

    test("memcmp", memcmp(a, b, 3) == 0);
    test("memcmp", memcmp(a, b, 4) == 1);
    test("memcmp", memcmp(b, a, 4) == -1);
}

WASM_EXPORT(test_memcpy)
void test_memcpy(void)
{
    char dest[] = { 1, 2, 3, 4 }, src[] = { 9, 8, 7 };
    char expected[] = { 9, 8, 3, 4 };
    memcpy(dest, src, 2);

    test("memcpy", memcmp(dest, expected, sizeof(expected)) == 0);
}

WASM_EXPORT(test_memset)
void test_memset(void)
{
    char s[] = { 9, 8, 7, 6 };
    char expected[] = { 1, 1, 1, 6 };
    memset(s, 1, 3);

    test("memset", memcmp(s, expected, sizeof(expected)) == 0);
}

int lex_crunch(const char *s)
{
    opa_json_lex ctx;
    opa_json_lex_init(s, opa_strlen(s), &ctx);
    return opa_json_lex_read(&ctx);
}

WASM_EXPORT(test_opa_lex_tokens)
void test_opa_lex_tokens(void)
{
    test("empty", lex_crunch("") == OPA_JSON_TOKEN_EOF);
    test("space", lex_crunch(" ") == OPA_JSON_TOKEN_EOF);
    test("tab", lex_crunch("\t") == OPA_JSON_TOKEN_EOF);
    test("newline", lex_crunch("\n") == OPA_JSON_TOKEN_EOF);
    test("carriage return", lex_crunch("\r") == OPA_JSON_TOKEN_EOF);
    test("null", lex_crunch("null") == OPA_JSON_TOKEN_NULL);
    test("true", lex_crunch("true") == OPA_JSON_TOKEN_TRUE);
    test("false", lex_crunch("false") == OPA_JSON_TOKEN_FALSE);

    test("bad unicode", lex_crunch("\" \\uabcx \"") == OPA_JSON_TOKEN_ERROR); // not hex
    test("escape not closed", lex_crunch("\"a\\\"") == OPA_JSON_TOKEN_ERROR); // unmatched escape
    test("bad escape character", lex_crunch("\"\\Q\"") == OPA_JSON_TOKEN_ERROR); // invalid escape character Q
    test("object start", lex_crunch(" { ") == OPA_JSON_TOKEN_OBJECT_START);
    test("object end", lex_crunch(" } ") == OPA_JSON_TOKEN_OBJECT_END);
    test("array start", lex_crunch(" [ ") == OPA_JSON_TOKEN_ARRAY_START);
    test("array end", lex_crunch(" ] ") == OPA_JSON_TOKEN_ARRAY_END);
    test("element separator", lex_crunch(" , ") == OPA_JSON_TOKEN_COMMA);
    test("item separator", lex_crunch(" : ") == OPA_JSON_TOKEN_COLON);
}

int lex_buffer_crunch(const char *s, const char *exp, int token)
{
    opa_json_lex ctx;
    opa_json_lex_init(s, opa_strlen(s), &ctx);

    if (opa_json_lex_read(&ctx) != token)
    {
        return -1;
    }

    size_t exp_len = opa_strlen(exp);
    size_t buf_len = ctx.buf_end - ctx.buf;

    if (exp_len != buf_len)
    {
        return -2;
    }

    if (opa_strncmp(ctx.buf, exp, buf_len) != 0)
    {
        return -3;
    }

    return 0;
}

#define test_lex_buffer(note, s, exp, token) test(note, (lex_buffer_crunch(s, exp, token) == 0))

WASM_EXPORT(test_opa_lex_buffer)
void test_opa_lex_buffer(void)
{
    test_lex_buffer("zero", "0", "0", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("signed zero", "-0", "-0", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("integers", "1234567890", "1234567890", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("signed integers", "-1234567890", "-1234567890", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("floats", "0.1234567890", "0.1234567890", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("signed floats", "-0.1234567890", "-0.1234567890", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("exponents", "-0.1234567890e0", "-0.1234567890e0", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("exponents", "-0.1234567890E+1000", "-0.1234567890E+1000", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("exponents", "-0.1234567890E-1000", "-0.1234567890E-1000", OPA_JSON_TOKEN_NUMBER);
    test_lex_buffer("empty string", "\"\"", "", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped buffer", "\"a\\\"b\"", "a\\\"b", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped quote", "\"\\\"\"", "\\\"", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped reverse solidus", "\"\\\\\"", "\\\\", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped solidus", "\"\\/\"", "\\/", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped backspace", "\"\\b\"", "\\b", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped feed forward", "\"\\f\"", "\\f", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped line feed", "\"\\n\"", "\\n", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped carriage return", "\"\\r\"", "\\r", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("escaped tab", "\"\\t\"", "\\t", OPA_JSON_TOKEN_STRING_ESCAPED);
    test_lex_buffer("plain", "\"abcdefg\"", "abcdefg", OPA_JSON_TOKEN_STRING);
}

WASM_EXPORT(test_opa_value_compare)
void test_opa_value_compare(void)
{
    test("none", opa_value_compare(NULL, NULL) == 0);
    test("none/some", opa_value_compare(NULL, opa_null()) < 0);
    test("some/none", opa_value_compare(opa_null(), NULL) > 0);
    test("null", opa_value_compare(opa_null(), opa_null()) == 0);
    test("null/boolean", opa_value_compare(opa_boolean(true), opa_null()) > 0);
    test("true/true", opa_value_compare(opa_boolean(true), opa_boolean(true)) == 0);
    test("true/false", opa_value_compare(opa_boolean(true), opa_boolean(false)) > 0);
    test("false/true", opa_value_compare(opa_boolean(false), opa_boolean(true)) < 0);
    test("false/false", opa_value_compare(opa_boolean(false), opa_boolean(false)) == 0);
    test("number/boolean", opa_value_compare(opa_number_int(100), opa_boolean(true)) > 0);
    test("integers", opa_value_compare(opa_number_int(100), opa_number_int(99)) > 0);
    test("integers", opa_value_compare(opa_number_int(100), opa_number_int(101)) < 0);
    test("integers", opa_value_compare(opa_number_int(100), opa_number_int(100)) == 0);
    test("integers", opa_value_compare(opa_number_int(-100), opa_number_int(100)) < 0);
    test("integers", opa_value_compare(opa_number_int(-100), opa_number_int(-101)) > 0);
    test("integer/float", opa_value_compare(opa_number_int(100), opa_number_float(100.1)) < 0);
    test("floats", opa_value_compare(opa_number_float(100.2), opa_number_float(100.1)) > 0);
    test("floats", opa_value_compare(opa_number_float(100.2), opa_number_float(100.3)) < 0);
    test("floats", opa_value_compare(opa_number_float(100.3), opa_number_float(100.3)) == 0);
    test("string/number", opa_value_compare(opa_string_terminated("foo"), opa_number_float(100)) > 0);
    test("strings", opa_value_compare(opa_string_terminated("foo"), opa_string_terminated("foo")) == 0);
    test("strings", opa_value_compare(opa_string_terminated("foo"), opa_string_terminated("bar")) > 0);
    test("strings", opa_value_compare(opa_string_terminated("bar"), opa_string_terminated("baz")) < 0);
    test("strings", opa_value_compare(opa_string_terminated("foobar"), opa_string_terminated("foo")) > 0);
    test("strings", opa_value_compare(opa_string_terminated("foo"), opa_string_terminated("foobar")) < 0);

    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_number_int(1));
    opa_array_append(arr1, opa_number_int(2));
    opa_array_append(arr1, opa_number_int(3));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, opa_number_int(1));
    opa_array_append(arr2, opa_number_int(3));
    opa_array_append(arr2, opa_number_int(2));

    opa_array_t *arr3 = opa_cast_array(opa_array());
    opa_array_append(arr2, opa_number_int(1));
    opa_array_append(arr2, opa_number_int(3));

    opa_value *v1, *v2, *v3;
    v1 = &arr1->hdr;
    v2 = &arr2->hdr;
    v3 = &arr3->hdr;

    test("array/string", opa_value_compare(v1, opa_string_terminated("a")) > 0);
    test("arrays", opa_value_compare(v1, v1) == 0);
    test("arrays", opa_value_compare(v1, v2) < 0);
    test("arrays", opa_value_compare(v2, v1) > 0);
    test("arrays", opa_value_compare(v3, v2) < 0);
    test("arrays", opa_value_compare(v2, v3) > 0);

    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj1, opa_string_terminated("b"), opa_number_int(2));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj2, opa_string_terminated("b"), opa_number_int(3));

    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj3, opa_string_terminated("c"), opa_number_int(3));

    opa_object_t *obj4 = opa_cast_object(opa_object());
    opa_object_insert(obj4, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj4, opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(obj4, opa_string_terminated("c"), opa_number_int(3));

    v1 = &obj1->hdr;
    v2 = &obj2->hdr;
    v3 = &obj3->hdr;
    opa_value *v4 = &obj4->hdr;

    test("object/array", opa_value_compare(v1, opa_array()) > 0);
    test("objects", opa_value_compare(v1, v1) == 0);
    test("objects", opa_value_compare(v1, v2) < 0);
    test("objects", opa_value_compare(v2, v3) < 0);
    test("objects", opa_value_compare(v4, v1) > 0);
    test("objects", opa_value_compare(v4, v2) < 0);

    opa_set_t *set1 = opa_cast_set(opa_set());
    opa_set_add(set1, opa_string_terminated("a"));
    opa_set_add(set1, opa_string_terminated("b"));

    opa_set_t *set2 = opa_cast_set(opa_set());
    opa_set_add(set2, opa_string_terminated("a"));
    opa_set_add(set2, opa_string_terminated("c"));

    opa_set_t *set3 = opa_cast_set(opa_set());
    opa_set_add(set3, opa_string_terminated("a"));
    opa_set_add(set3, opa_string_terminated("b"));
    opa_set_add(set3, opa_string_terminated("c"));

    v1 = &set1->hdr;
    v2 = &set2->hdr;
    v3 = &set3->hdr;

    test("set/object", opa_value_compare(v1, opa_object()) > 0);
    test("sets", opa_value_compare(v1, v1) == 0);
    test("sets", opa_value_compare(v1, v2) < 0);
    test("sets", opa_value_compare(v2, v3) > 0); // because c > b
    test("sets", opa_value_compare(v3, v1) > 0);
}

int parse_crunch(const char *s, opa_value *exp)
{
    opa_value *ret = opa_json_parse(s, opa_strlen(s));
    if (ret == NULL)
    {
        return 0;
    }
    return opa_value_compare(exp, ret) == 0;
}

int value_parse_crunch(const char *s, opa_value *exp)
{
    opa_value *ret = opa_value_parse(s, opa_strlen(s));
    if (ret == NULL)
    {
        return 0;
    }
    return opa_value_compare(exp, ret) == 0;
}

WASM_EXPORT(test_opa_json_parse_scalar)
void test_opa_json_parse_scalar(void)
{
    test("null", parse_crunch("null", opa_null()));
    test("true", parse_crunch("true", opa_boolean(true)));
    test("false", parse_crunch("false", opa_boolean(false)));
    test("strings", parse_crunch("\"hello\"", opa_string_terminated("hello")));
    test("strings: escaped quote", parse_crunch("\"a\\\"b\"", opa_string_terminated("a\"b")));
    test("strings: escaped reverse solidus", parse_crunch("\"a\\\\b\"", opa_string_terminated("a\\b")));
    test("strings: escaped solidus", parse_crunch("\"a\\/b\"", opa_string_terminated("a/b")));
    test("strings: escaped backspace", parse_crunch("\"a\\bb\"", opa_string_terminated("a\bb")));
    test("strings: escaped feed forward", parse_crunch("\"a\\fb\"", opa_string_terminated("a\fb")));
    test("strings: escaped line feed", parse_crunch("\"a\\nb\"", opa_string_terminated("a\nb")));
    test("strings: escaped carriage return", parse_crunch("\"a\\rb\"", opa_string_terminated("a\rb")));
    test("strings: escaped tab", parse_crunch("\"a\\tb\"", opa_string_terminated("a\tb")));
    test("strings: utf-8 2 bytes", parse_crunch("\"\xc2\xa2\"", opa_string_terminated("\xc2\xa2")));
    test("strings: utf-8 3 bytes", parse_crunch("\"\xe0\xb8\x81\"", opa_string_terminated("\xe0\xb8\x81")));
    test("strings: utf-8 3 bytes", parse_crunch("\"\xe2\x82\xac\"", opa_string_terminated("\xe2\x82\xac")));
    test("strings: utf-8 3 bytes", parse_crunch("\"\xed\x9e\xb0\"", opa_string_terminated("\xed\x9e\xb0")));
    test("strings: utf-8 3 bytes", parse_crunch("\"\xef\xa4\x80\"", opa_string_terminated("\xef\xa4\x80")));
    test("strings: utf-8 4 bytes", parse_crunch("\"\xf0\x90\x8d\x88\"", opa_string_terminated("\xf0\x90\x8d\x88")));
    test("strings: utf-8 4 bytes", parse_crunch("\"\xf3\xa0\x80\x81\"", opa_string_terminated("\xf3\xa0\x80\x81")));
    test("strings: utf-8 4 bytes", parse_crunch("\"\xf4\x80\x80\x80\"", opa_string_terminated("\xf4\x80\x80\x80")));
    test("strings: utf-16 no surrogate pair", parse_crunch("\" \\u20AC \"", opa_string_terminated(" \xe2\x82\xac ")));
    test("strings: utf-16 surrogate pair", parse_crunch("\" \\ud801\\udc37 \"", opa_string_terminated(" \xf0\x90\x90\xb7 ")));
    test("integers", parse_crunch("0", opa_number_int(0)));
    test("integers", parse_crunch("123456789", opa_number_int(123456789)));
    test("signed integers", parse_crunch("-0", opa_number_int(0)));
    test("signed integers", parse_crunch("-123456789", opa_number_int(-123456789)));
    test("floats", parse_crunch("16.7", opa_number_float(16.7)));
    test("signed floats", parse_crunch("-16.7", opa_number_float(-16.7)));
    test("exponents", parse_crunch("6e7", opa_number_float(6e7)));
}

WASM_EXPORT(test_opa_json_max_str_len)
void test_opa_json_max_str_len(void)
{
    test("max str len: a char", opa_json_max_string_len("a", 1) == 1);
    test("max str len: chars", opa_json_max_string_len("ab", 2) == 2);
    test("max str len: single char escape", opa_json_max_string_len("ab\nd", 4) == 4);
    test("max str len: 2 byte utf-8", opa_json_max_string_len("\xc2\xa2", 2) == 2);
    test("max str len: 3 byte utf-8", opa_json_max_string_len("\xe0\xb8\x81", 3) == 3);
    test("max str len: 4 byte utf-8", opa_json_max_string_len("\xf0\x90\x8d\x88", 4) == 4);
    test("max str len: utf-16 no surrogate pair", opa_json_max_string_len(" \\u20AC ", 8) == 6);
    test("max str len: utf-16 surrogate pair", opa_json_max_string_len(" \\ud801\\udc37 ", 14) == 6);
}

opa_array_t *fixture_array1(void)
{
    opa_array_t *arr = opa_cast_array(opa_array());
    opa_array_append(arr, opa_number_int(1));
    opa_array_append(arr, opa_number_int(2));
    opa_array_append(arr, opa_number_int(3));
    opa_array_append(arr, opa_number_int(4));
    return arr;
}

opa_array_t *fixture_array2(void)
{
    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_number_int(1));
    opa_array_append(arr1, opa_number_int(2));
    opa_array_append(arr1, opa_number_int(3));
    opa_array_append(arr1, opa_number_int(4));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, opa_number_int(5));
    opa_array_append(arr2, opa_number_int(6));
    opa_array_append(arr2, opa_number_int(7));
    opa_array_append(arr2, opa_number_int(8));

    opa_array_t *arr = opa_cast_array(opa_array());
    opa_array_append(arr, &arr1->hdr);
    opa_array_append(arr, &arr2->hdr);

    return arr;
}

opa_object_t *fixture_object1(void)
{
    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(2));
    return obj;
}

opa_object_t *fixture_object2(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(1));
    opa_object_insert(obj1, opa_string_terminated("d"), opa_number_int(2));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("e"), opa_number_int(3));
    opa_object_insert(obj2, opa_string_terminated("f"), opa_number_int(4));

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj1->hdr);
    opa_object_insert(obj, opa_string_terminated("b"), &obj2->hdr);
    return obj;
}

opa_set_t *fixture_set1(void)
{
    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_string_terminated("a"));
    opa_set_add(set, opa_string_terminated("b"));
    return set;
}

WASM_EXPORT(test_opa_value_length)
void test_opa_value_length(void)
{
    opa_array_t *arr = fixture_array1();
    opa_object_t *obj = fixture_object1();
    opa_set_t *set = fixture_set1();

    test("arrays", opa_value_length(&arr->hdr) == 4);
    test("objects", opa_value_length(&obj->hdr) == 2);
    test("sets", opa_value_length(&set->hdr) == 2);
}

WASM_EXPORT(test_opa_value_get_array)
void test_opa_value_get_array(void)
{
    opa_array_t *arr = fixture_array1();

    for (int i = 0; i < 4; i++)
    {
        opa_value *result = opa_value_get(&arr->hdr, opa_number_int(i));

        if (result == NULL)
        {
            test_fatal("array get failed");
        }

        if (opa_value_compare(result, opa_number_int(i + 1)) != 0)
        {
            test_fatal("array get returned bad value");
        }
    }

    opa_value *result = opa_value_get(&arr->hdr, opa_string_terminated("foo"));

    if (result != NULL)
    {
        test_fatal("array get returned unexpected result");
    }

    result = opa_value_get(&arr->hdr, opa_number_float(3.14));

    if (result != NULL)
    {
        test_fatal("array get returned unexpected result");
    }

    result = opa_value_get(&arr->hdr, opa_number_int(-1));

    if (result != NULL)
    {
        test_fatal("array get returned unexpected result");
    }

    result = opa_value_get(&arr->hdr, opa_number_int(4));

    if (result != NULL)
    {
        test_fatal("array get returned unexpected result");
    }
}

WASM_EXPORT(test_opa_array_sort)
void test_opa_array_sort(void)
{
    opa_array_t *arr = opa_cast_array(opa_array());

    opa_array_append(arr, opa_number_int(4));
    opa_array_append(arr, opa_number_int(3));
    opa_array_append(arr, opa_number_int(2));
    opa_array_append(arr, opa_number_int(1));

    opa_array_sort(arr, opa_value_compare);

    // iterate through the array to verify both the indices and values.

    opa_value *res = opa_array();
    opa_value *exp = &fixture_array1()->hdr;

    for (opa_value *prev = NULL, *curr = NULL; (curr = opa_value_iter(&arr->hdr, prev)) != NULL; prev = curr)
    {
        opa_array_append(opa_cast_array(res), opa_value_get(&arr->hdr, curr));
    }

    if (opa_value_compare(res, exp) != 0)
    {
        test_fatal("array sort returned unexpected result");
    }
}

WASM_EXPORT(test_opa_value_get_object)
void test_opa_value_get_object(void)
{
    opa_object_t *obj = fixture_object1();

    const char *keys[2] = {
        "a",
        "b",
    };

    long long values[2] = {
        1,
        2,
    };

    for (int i = 0; i < sizeof(keys) / sizeof(const char *); i++)
    {
        opa_value *result = opa_value_get(&obj->hdr, opa_string_terminated(keys[i]));

        if (result == NULL)
        {
            test_fatal("object get failed");
        }

        if (opa_value_compare(result, opa_number_int(values[i])) != 0)
        {
            test_fatal("object get returned bad value");
        }
    }

    opa_value *result = opa_value_get(&obj->hdr, opa_string_terminated("non-existent"));

    if (result != NULL)
    {
        test_fatal("object get returned unexpected result");
    }
}

WASM_EXPORT(test_opa_json_parse_composites)
void test_opa_json_parse_composites(void)
{

    opa_value *empty_arr = opa_array();

    test("empty array", parse_crunch("[]", empty_arr));
    test("array", parse_crunch("[1,2,3,4]", &fixture_array1()->hdr));
    test("array nested", parse_crunch("[[1,2,3,4],[5,6,7,8]]", &fixture_array2()->hdr));

    opa_value *empty_obj = opa_object();

    test("empty object", parse_crunch("{}", empty_obj));
    test("object", parse_crunch("{\"a\": 1, \"b\": 2}", &fixture_object1()->hdr));
    test("object nested", parse_crunch("{\"a\": {\"c\": 1, \"d\": 2}, \"b\": {\"e\": 3, \"f\": 4}}", &fixture_object2()->hdr));
}

WASM_EXPORT(test_opa_value_parse)
void test_opa_value_parse(void)
{
    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_number_int(1));

    test("set of one", value_parse_crunch("{1}", &set->hdr));
    test("set of one - dupes", value_parse_crunch("{1,1}", &set->hdr));

    opa_set_add(set, opa_number_int(2));
    opa_set_add(set, opa_number_int(3));

    test("set multiple", value_parse_crunch("{1,2,3}", &set->hdr));

    opa_value *empty_set = opa_set();

    test("empty", value_parse_crunch("set()", empty_set));
    test("empty whitespace", value_parse_crunch("set(   )", empty_set));
}

WASM_EXPORT(test_opa_json_parse_memory_ownership)
void test_opa_json_parse_memory_ownership(void)
{
    char s[] = "[1,\"a\"]";

    opa_value *result = opa_json_parse(s, sizeof(s));

    opa_value *exp = opa_array();
    opa_array_t *arr = opa_cast_array(exp);
    opa_array_append(arr, opa_number_int(1));
    opa_array_append(arr, opa_string("a", 1));

    test("expected value", opa_value_compare(result, exp) == 0);

    for (int i = 0; i < sizeof(s); i++)
    {
        s[i] = 0;
    }

    test("expected value after overwriting buffer", opa_value_compare(result, exp) == 0);
}

WASM_EXPORT(test_opa_object_insert)
void test_opa_object_insert(void)
{

    opa_object_t *obj = opa_cast_object(opa_object());

    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(3));

    opa_value *v1 = opa_value_get(&obj->hdr, opa_string_terminated("a"));

    if (opa_value_compare(v1, opa_number_int(3)) != 0)
    {
        test_fatal("object insert did not replace value")
    }

    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(4));

    opa_value *v2 = opa_value_get(&obj->hdr, opa_string_terminated("b"));

    if (opa_value_compare(v2, opa_number_int(4)) != 0)
    {
        test_fatal("object insert did not replace value")
    }
}

WASM_EXPORT(test_opa_object_growth)
void test_opa_object_growth(void)
{
    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), opa_string_terminated("1"));
    opa_object_insert(obj, opa_string_terminated("b"), opa_string_terminated("2"));
    opa_object_insert(obj, opa_string_terminated("c"), opa_string_terminated("3"));
    opa_object_insert(obj, opa_string_terminated("d"), opa_string_terminated("4"));
    opa_object_insert(obj, opa_string_terminated("e"), opa_string_terminated("5"));
    opa_object_insert(obj, opa_string_terminated("e"), opa_string_terminated("5'"));

    if (obj->len != 5)
    {
        test_fatal("object is missing key-value pairs")
    }

    if (obj->n != 8)
    {
        test_fatal("object capacity did double")
    }

    opa_object_insert(obj, opa_string_terminated("f"), opa_string_terminated("6"));

    if (obj->len != 6)
    {
        test_fatal("object is missing key-value pairs")
    }

    if (obj->n != 16)
    {
        test_fatal("object capacity did not double")
    }
}

WASM_EXPORT(test_opa_set_add_and_get)
void test_opa_set_add_and_get(void)
{
    opa_set_t *set = fixture_set1();
    opa_set_add(set, opa_string_terminated("a"));

    opa_set_t *cpy = fixture_set1();

    if (opa_value_compare(&set->hdr, &cpy->hdr) != 0)
    {
        test_fatal("set was modified by add with duplicate element");
    }

    opa_set_add(set, opa_string_terminated("c"));

    if (opa_value_compare(&set->hdr, &cpy->hdr) <= 0)
    {
        test_fatal("set should be greater than cpy")
    }

    if (opa_value_get(&set->hdr, opa_string_terminated("c")) == NULL)
    {
        test_fatal("set should contain string term c")
    }

    opa_set_t *order = opa_cast_set(opa_set());
    opa_set_add(order, opa_string_terminated("b"));
    opa_set_add(order, opa_string_terminated("c"));
    opa_set_add(order, opa_string_terminated("a"));

    if (opa_value_compare(&set->hdr, &order->hdr) != 0)
    {
        test_fatal("sets should be equal")
    }
}

WASM_EXPORT(test_opa_set_growth)
void test_opa_set_growth(void)
{
    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_string_terminated("a"));
    opa_set_add(set, opa_string_terminated("b"));
    opa_set_add(set, opa_string_terminated("c"));
    opa_set_add(set, opa_string_terminated("d"));
    opa_set_add(set, opa_string_terminated("e"));
    opa_set_add(set, opa_string_terminated("e"));

    if (set->len != 5)
    {
        test_fatal("set is missing elements")
    }

    if (set->n != 8)
    {
        test_fatal("set capacity did double")
    }

    opa_set_add(set, opa_string_terminated("f"));

    if (set->len != 6)
    {
        test_fatal("set is missing elements")
    }

    if (set->n != 16)
    {
        test_fatal("set capacity did not double")
    }
}

WASM_EXPORT(test_opa_value_iter_object)
void test_opa_value_iter_object(void)
{
    opa_object_t *obj = fixture_object1();

    opa_value *k1 = opa_value_iter(&obj->hdr, NULL);
    opa_value *k2 = opa_value_iter(&obj->hdr, k1);
    opa_value *k3 = opa_value_iter(&obj->hdr, k2);

    opa_value *exp1 = opa_string_terminated("b");
    opa_value *exp2 = opa_string_terminated("a");
    opa_value *exp3 = NULL;

    if (opa_value_compare(k1, exp1) != 0)
    {
        test_fatal("object iter start did not return expected value");
    }

    if (opa_value_compare(k2, exp2) != 0)
    {
        test_fatal("object iter second did not return expected value");
    }

    if (opa_value_compare(k3, exp3) != 0)
    {
        test_fatal("object iter third did not return expected value");
    }
}

WASM_EXPORT(test_opa_value_iter_array)
void test_opa_value_iter_array(void)
{
    opa_array_t *arr = opa_cast_array(opa_array());

    opa_array_append(arr, opa_number_int(1));
    opa_array_append(arr, opa_number_int(2));

    opa_value *k1 = opa_value_iter(&arr->hdr, NULL);
    opa_value *k2 = opa_value_iter(&arr->hdr, k1);
    opa_value *k3 = opa_value_iter(&arr->hdr, k2);

    opa_value *exp1 = opa_number_int(0);
    opa_value *exp2 = opa_number_int(1);
    opa_value *exp3 = NULL;

    if (opa_value_compare(k1, exp1) != 0)
    {
        test_fatal("array iter start did not return expected value");
    }

    if (opa_value_compare(k2, exp2) != 0)
    {
        test_fatal("array iter second did not return expected value");
    }

    if (opa_value_compare(k3, exp3) != 0)
    {
        test_fatal("array iter third did not return expected value");
    }
}

WASM_EXPORT(test_opa_value_iter_set)
void test_opa_value_iter_set(void)
{
    opa_set_t *set = opa_cast_set(opa_set());

    opa_set_add(set, opa_number_int(1));
    opa_set_add(set, opa_number_int(2));

    opa_value *v1 = opa_value_iter(&set->hdr, NULL);
    opa_value *v2 = opa_value_iter(&set->hdr, v1);
    opa_value *v3 = opa_value_iter(&set->hdr, v2);

    opa_value *exp1 = opa_number_int(1);
    opa_value *exp2 = opa_number_int(2);
    opa_value *exp3 = NULL;

    if (opa_value_compare(v1, exp1) != 0)
    {
        test_fatal("set iter did not return expected value");
    }

    if (opa_value_compare(v2, exp2) != 0)
    {
        test_fatal("set iter second did not return expected value");
    }

    if (opa_value_compare(v3, exp3) != 0)
    {
        test_fatal("set iter third did not return expected value");
    }
}

WASM_EXPORT(test_opa_value_merge_scalars)
void test_opa_value_merge_scalars(void)
{
    opa_value *result = opa_value_merge(opa_number_int(1), opa_string_terminated("foo"));

    if (result == NULL)
    {
        test_fatal("merge of two scalars failed");
    }
    else if (opa_value_compare(result, opa_number_int(1)) != 0)
    {
        test_fatal("scalar merge returned unexpected result");
    }
}

WASM_EXPORT(test_opa_value_merge_first_operand_null)
void test_opa_value_merge_first_operand_null(void)
{
    test_str_eq("second operand string returns string", "\"foo\"", opa_json_dump(opa_value_merge(NULL, opa_string_terminated("foo"))));
    test_str_eq("second operand object returns object", "{}", opa_json_dump(opa_value_merge(NULL, opa_object())));
    test_str_eq("second operand number returns number", "1", opa_json_dump(opa_value_merge(NULL, opa_number_int(1))));
    test_str_eq("second operand array returns array", "[]", opa_json_dump(opa_value_merge(NULL, opa_array())));
    test_str_eq("second operand set returns set", "set()", opa_value_dump(opa_value_merge(NULL, opa_set())));
    test_str_eq("second operand null returns null", "null", opa_json_dump(opa_value_merge(NULL, opa_null())));
}

WASM_EXPORT(test_opa_value_merge_simple)
void test_opa_value_merge_simple(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_t *obj2 = opa_cast_object(opa_object());

    opa_object_insert(obj1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj2, opa_string_terminated("b"), opa_number_int(2));

    opa_object_t *exp1 = opa_cast_object(opa_object());
    opa_object_insert(exp1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(exp1, opa_string_terminated("b"), opa_number_int(2));

    opa_value *result = opa_value_merge(&obj1->hdr, &obj2->hdr);

    if (result == NULL)
    {
        test_fatal("object merge failed");
    }
    else if (opa_value_compare(result, &exp1->hdr) != 0)
    {
        test_fatal("object merge returned unexpected result");
    }
}


WASM_EXPORT(test_opa_value_merge_nested)
void test_opa_value_merge_nested(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_t *obj1a = opa_cast_object(opa_object());

    opa_object_insert(obj1a, opa_string_terminated("b"), opa_number_int(1));
    opa_object_insert(obj1, opa_string_terminated("a"), &obj1a->hdr);
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(2));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_t *obj2a = opa_cast_object(opa_object());

    opa_object_insert(obj2a, opa_string_terminated("d"), opa_number_int(3));
    opa_object_insert(obj2, opa_string_terminated("a"), &obj2a->hdr);
    opa_object_insert(obj2, opa_string_terminated("e"), opa_number_int(4));

    opa_object_t *exp1 = opa_cast_object(opa_object());
    opa_object_t *exp1a = opa_cast_object(opa_object());

    opa_object_insert(exp1a, opa_string_terminated("b"), opa_number_int(1));
    opa_object_insert(exp1a, opa_string_terminated("d"), opa_number_int(3));
    opa_object_insert(exp1, opa_string_terminated("a"), &exp1a->hdr);
    opa_object_insert(exp1, opa_string_terminated("c"), opa_number_int(2));
    opa_object_insert(exp1, opa_string_terminated("e"), opa_number_int(4));

    opa_value *result = opa_value_merge(&obj1->hdr, &obj2->hdr);

    if (result == NULL)
    {
        test_fatal("object merge failed");
    }
    else if (opa_value_compare(&exp1->hdr, result) != 0)
    {
        test_fatal("object merge returned unexpected result");
    }
}

WASM_EXPORT(test_opa_value_shallow_copy)
void test_opa_value_shallow_copy(void)
{
    // construct a value that has one of each type
    char str[] = "{\"a\": [1, true, null, 2.5]}";
    opa_value *obj = opa_json_parse(str, sizeof(str));
    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, obj);

    opa_value *cpy = opa_value_shallow_copy(&set->hdr);

    if (opa_value_compare(cpy, &set->hdr) != 0)
    {
        test_fatal("expected original and shallow copy to be equal");
    }
}

WASM_EXPORT(test_opa_json_dump)
void test_opa_json_dump(void)
{
    test("null", opa_strcmp(opa_json_dump(opa_null()), "null") == 0);
    test("false", opa_strcmp(opa_json_dump(opa_boolean(false)), "false") == 0);
    test("true", opa_strcmp(opa_json_dump(opa_boolean(true)), "true") == 0);
    test("strings", opa_strcmp(opa_json_dump(opa_string_terminated("hello\"world")), "\"hello\\\"world\"") == 0);
    test("strings utf-8", opa_strcmp(opa_json_dump(opa_string_terminated("\xed\xba\xad")), "\"\xed\xba\xad\"") == 0);
    test("numbers", opa_strcmp(opa_json_dump(opa_number_int(127)), "127") == 0);

    test_str_eq("numbers/float", "12345.678", opa_json_dump(opa_number_float(12345.678)));
    test_str_eq("numbers/float", "10.5", opa_json_dump(opa_number_float(10.5)));

    opa_value *arr = opa_array();
    test("arrays", opa_strcmp(opa_json_dump(arr), "[]") == 0);

    opa_array_append(opa_cast_array(arr), opa_string_terminated("hello"));
    test("arrays", opa_strcmp(opa_json_dump(arr), "[\"hello\"]") == 0);

    opa_array_append(opa_cast_array(arr), opa_string_terminated("world"));
    test("arrays", opa_strcmp(opa_json_dump(arr), "[\"hello\",\"world\"]") == 0);

    opa_value *set = opa_set();
    test("sets", opa_strcmp(opa_json_dump(set), "[]") == 0);

    opa_set_add(opa_cast_set(set), opa_string_terminated("hello"));
    test("sets", opa_strcmp(opa_json_dump(set), "[\"hello\"]") == 0);

    opa_set_add(opa_cast_set(set), opa_string_terminated("world"));
    test("sets", opa_strcmp(opa_json_dump(set), "[\"hello\",\"world\"]") == 0);

    opa_value *obj = opa_object();
    test("objects", opa_strcmp(opa_json_dump(obj), "{}") == 0);

    opa_object_insert(opa_cast_object(obj), opa_string_terminated("k1"), opa_string_terminated("v1"));
    test("objects", opa_strcmp(opa_json_dump(obj), "{\"k1\":\"v1\"}") == 0);

    opa_object_insert(opa_cast_object(obj), opa_string_terminated("k2"), opa_string_terminated("v2"));
    test("objects", opa_strcmp(opa_json_dump(obj), "{\"k1\":\"v1\",\"k2\":\"v2\"}") == 0);

    opa_value *terminators = opa_array();
    opa_array_append(opa_cast_array(terminators), opa_boolean(true));
    opa_array_append(opa_cast_array(terminators), opa_boolean(false));
    opa_array_append(opa_cast_array(terminators), opa_null());

    test("bool/null terminators", opa_strcmp(opa_json_dump(terminators), "[true,false,null]") == 0);

    opa_value *non_string_keys = opa_object();
    opa_array_t *arrk = opa_cast_array(opa_array());
    opa_array_append(arrk, opa_number_int(1));
    opa_object_insert(opa_cast_object(non_string_keys), &arrk->hdr, opa_number_int(1));
    test_str_eq("objects/non string keys", opa_json_dump(non_string_keys), "{\"[1]\":1}");
}

WASM_EXPORT(test_opa_value_dump)
void test_opa_value_dump(void)
{
    test("empty sets", opa_strcmp(opa_value_dump(opa_set()), "set()") == 0);

    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_number_int(1));

    test("sets of one", opa_strcmp(opa_value_dump(&set->hdr), "{1}") == 0);

    opa_set_add(set, opa_number_int(2));
    test("sets", opa_strcmp(opa_value_dump(&set->hdr), "{1,2}") == 0);

    opa_value *non_string_keys = opa_object();
    opa_array_t *arrk = opa_cast_array(opa_array());
    opa_array_append(arrk, opa_number_int(1));
    opa_object_insert(opa_cast_object(non_string_keys), &arrk->hdr, opa_number_int(1));
    test_str_eq("objects/non string keys", opa_value_dump(non_string_keys), "{[1]:1}");
}

WASM_EXPORT(test_arithmetic)
void test_arithmetic(void)
{
    long long i = 0;

    test("abs +1", opa_number_try_int(opa_cast_number(opa_arith_abs(opa_number_int(1))), &i) == 0 && i == 1);
    test("abs -1", opa_number_try_int(opa_cast_number(opa_arith_abs(opa_number_int(-1))), &i) == 0 && i == 1);
    test("abs 1.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_abs(opa_number_float(1.5)))) == 1.5);
    test("abs -1.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_abs(opa_number_float(-1.5)))) == 1.5);
    test("abs 1.5 (ref)", opa_number_as_float(opa_cast_number(opa_arith_abs(opa_number_ref("1.5", 3)))) == 1.5);
    test("abs -1.5 (ref)", opa_number_as_float(opa_cast_number(opa_arith_abs(opa_number_ref("-1.5", 4)))) == 1.5);
    test("round 1", opa_number_try_int(opa_cast_number(opa_arith_round(opa_number_int(1))), &i) == 0 && i == 1);
    test("round -1", opa_number_try_int(opa_cast_number(opa_arith_round(opa_number_int(-1))), &i) == 0 && i == -1);
    test("round 1.4 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(1.4)))) == 1);
    test("round -1.4 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(-1.4)))) == -1);
    test("round 1.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(1.5)))) == 2);
    test("round -1.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(-1.5)))) == -2);
    test("round 2.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(2.5)))) == 3);
    test("round -2.5 (float)", opa_number_as_float(opa_cast_number(opa_arith_round(opa_number_float(-2.5)))) == -3);
    test("ceil 1", opa_number_as_float(opa_cast_number(opa_arith_ceil(opa_number_int(1)))) == 1);
    test("ceil 1.01 (float)", opa_number_as_float(opa_cast_number(opa_arith_ceil(opa_number_float(1.01)))) == 2);
    test("ceil -1.99999 (float)", opa_number_as_float(opa_cast_number(opa_arith_ceil(opa_number_float(-1.99999)))) == -1);
    test("floor 1", opa_number_as_float(opa_cast_number(opa_arith_floor(opa_number_int(1)))) == 1);
    test("floor 1.01 (float)", opa_number_as_float(opa_cast_number(opa_arith_floor(opa_number_float(1.01)))) == 1);
    test("floor -1.99999 (float)", opa_number_as_float(opa_cast_number(opa_arith_floor(opa_number_float(-1.99999)))) == -2);
    test("plus 1+2", opa_number_as_float(opa_cast_number(opa_arith_plus(opa_number_float(1), opa_number_float(2)))) == 3);
    test("minus 3-2", opa_number_as_float(opa_cast_number(opa_arith_minus(opa_number_float(3), opa_number_float(2)))) == 1);

    opa_set_t *s1 = opa_cast_set(opa_set());
    opa_set_add(s1, opa_number_int(0));
    opa_set_add(s1, opa_number_int(1));
    opa_set_add(s1, opa_number_int(2));

    opa_set_t *s2 = opa_cast_set(opa_set());
    opa_set_add(s2, opa_number_int(0));
    opa_set_add(s2, opa_number_int(2));

    opa_set_t *s3 = opa_cast_set(opa_arith_minus(&s1->hdr, &s2->hdr));
    test("minus set", s3->len == 1 && opa_set_get(s3, opa_number_int(1)) != NULL);
    test("multiply 3*2", opa_number_as_float(opa_cast_number(opa_arith_multiply(opa_number_float(3), opa_number_float(2)))) == 6);
    test("divide 3/2", opa_number_as_float(opa_cast_number(opa_arith_divide(opa_number_float(3), opa_number_float(2)))) == 1.5);
    test("divide 3/0", opa_arith_divide(opa_number_float(3), opa_number_float(0)) == NULL);
    test("remainder 5 % 2", opa_number_as_float(opa_cast_number(opa_arith_rem(opa_number_float(5), opa_number_float(2)))) == 1);
    test("remainder 1.1 % 1", opa_arith_rem(opa_number_float(1.1), opa_number_float(1)) == NULL);
    test("remainder 1 % 1.1", opa_arith_rem(opa_number_float(1), opa_number_float(1.1)) == NULL);
    test("remainder 1 % 0", opa_arith_rem(opa_number_float(1), opa_number_float(0)) == NULL);
}

WASM_EXPORT(test_set_diff)
void test_set_diff(void)
{
    // test_arithmetic covers the diff.
}

WASM_EXPORT(test_set_intersection_union)
void test_set_intersection_union(void)
{
    opa_set_t *s1 = opa_cast_set(opa_set());
    opa_set_add(s1, opa_number_int(0));
    opa_set_add(s1, opa_number_int(1));
    opa_set_add(s1, opa_number_int(2));

    opa_set_t *s2 = opa_cast_set(opa_set());
    opa_set_add(s2, opa_number_int(0));
    opa_set_add(s2, opa_number_int(1));

    opa_set_t *r = opa_cast_set(opa_set_intersection(&s1->hdr, &s2->hdr));
    test("set/intersection", r->len == 2 && opa_set_get(r, opa_number_int(0)) != NULL && opa_set_get(r, opa_number_int(1)) != NULL);

    r = opa_cast_set(opa_set_union(&s1->hdr, &s2->hdr));
    test("set/union", r->len == 3 &&
         opa_set_get(r, opa_number_int(0)) != NULL &&
         opa_set_get(r, opa_number_int(1)) != NULL &&
         opa_set_get(r, opa_number_int(2)) != NULL);
}


WASM_EXPORT(test_sets_intersection_union)
void test_sets_intersection_union(void)
{
    opa_set_t *s1 = opa_cast_set(opa_set());
    opa_set_add(s1, opa_number_int(0));
    opa_set_add(s1, opa_number_int(1));
    opa_set_add(s1, opa_number_int(2));

    opa_set_t *s2 = opa_cast_set(opa_set());
    opa_set_add(s2, opa_number_int(0));
    opa_set_add(s2, opa_number_int(1));

    opa_set_t *s3 = opa_cast_set(opa_set());
    opa_set_add(s3, opa_number_int(0));

    opa_set_t *sets = opa_cast_set(opa_set());
    opa_set_add(sets, &s1->hdr);
    opa_set_add(sets, &s2->hdr);
    opa_set_add(sets, &s3->hdr);

    opa_set_t *r = opa_cast_set(opa_sets_intersection(&sets->hdr));
    test("sets/intersection", r->len == 1 && opa_set_get(r, opa_number_int(0)) != NULL);

    r = opa_cast_set(opa_sets_union(&sets->hdr));
    test("sets/union", r->len == 3 &&
         opa_set_get(r, opa_number_int(0)) != NULL &&
         opa_set_get(r, opa_number_int(1)) != NULL &&
         opa_set_get(r, opa_number_int(2)) != NULL);
}

WASM_EXPORT(test_array)
void test_array(void)
{
    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_number_int(0));
    opa_array_append(arr1, opa_number_int(1));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, opa_number_int(2));
    opa_array_append(arr2, opa_number_int(3));

    opa_array_t *r = opa_cast_array(opa_array_concat(&arr1->hdr, &arr2->hdr));

    test("array_concat", r->len == 4 &&
         opa_value_compare(r->elems[0].v, opa_number_int(0)) == 0 &&
         opa_value_compare(r->elems[1].v, opa_number_int(1)) == 0 &&
         opa_value_compare(r->elems[2].v, opa_number_int(2)) == 0 &&
         opa_value_compare(r->elems[3].v, opa_number_int(3)) == 0);

    r = opa_cast_array(opa_array_slice(&r->hdr, opa_number_int(1), opa_number_int(3)));

    test("array_slice", r->len == 2 &&
         opa_value_compare(r->elems[0].v, opa_number_int(1)) == 0 &&
         opa_value_compare(r->elems[1].v, opa_number_int(2)) == 0);
    
    opa_array_t *arr3 = opa_cast_array(opa_array());
    opa_array_append(arr3, opa_number_int(0));
    opa_array_append(arr3, opa_number_int(1));
    opa_array_append(arr3, opa_number_int(2));

    r = opa_cast_array(opa_array_reverse(&arr3->hdr));
    test("array_reverse", r->len == 3 &&
         opa_value_compare(r->elems[0].v, opa_number_int(2)) == 0 &&
         opa_value_compare(r->elems[1].v, opa_number_int(1)) == 0 &&
         opa_value_compare(r->elems[2].v, opa_number_int(0)) == 0);
}

WASM_EXPORT(test_types)
void test_types(void)
{
    test("is_number", opa_value_compare(opa_types_is_number(opa_number_int(0)), opa_boolean(true)) == 0);
    test("is_number", opa_value_compare(opa_types_is_number(opa_null()), opa_boolean(false)) == 0);
    test("is_string", opa_value_compare(opa_types_is_string(opa_string("a", 1)), opa_boolean(true)) == 0);
    test("is_string", opa_value_compare(opa_types_is_string(opa_null()), opa_boolean(false)) == 0);
    test("is_boolean", opa_value_compare(opa_types_is_boolean(opa_boolean(true)), opa_boolean(true)) == 0);
    test("is_boolean", opa_value_compare(opa_types_is_boolean(opa_null()), opa_boolean(false)) == 0);
    test("is_array", opa_value_compare(opa_types_is_array(opa_array()), opa_boolean(true)) == 0);
    test("is_array", opa_value_compare(opa_types_is_array(opa_null()), opa_boolean(false)) == 0);
    test("is_set", opa_value_compare(opa_types_is_set(opa_set()), opa_boolean(true)) == 0);
    test("is_set", opa_value_compare(opa_types_is_set(opa_null()), opa_boolean(false)) == 0);
    test("is_object", opa_value_compare(opa_types_is_object(opa_object()), opa_boolean(true)) == 0);
    test("is_object", opa_value_compare(opa_types_is_object(opa_null()), opa_boolean(false)) == 0);
    test("is_null", opa_value_compare(opa_types_is_null(opa_null()), opa_boolean(true)) == 0);
    test("is_null", opa_value_compare(opa_types_is_null(opa_number_int(0)), opa_boolean(false)) == 0);

    test("name/null", opa_value_compare(opa_types_name(opa_null()), opa_string("null", 4)) == 0);
    test("name/boolean", opa_value_compare(opa_types_name(opa_boolean(true)), opa_string("boolean", 7)) == 0);
    test("name/number", opa_value_compare(opa_types_name(opa_number_int(0)), opa_string("number", 6)) == 0);
    test("name/string", opa_value_compare(opa_types_name(opa_string("a", 1)), opa_string("string", 6)) == 0);
    test("name/array", opa_value_compare(opa_types_name(opa_array()), opa_string("array", 5)) == 0);
    test("name/object", opa_value_compare(opa_types_name(opa_object()), opa_string("object", 6)) == 0);
    test("name/set", opa_value_compare(opa_types_name(opa_set()), opa_string("set", 3)) == 0);
}

static opa_value *number(const char *s)
{
    size_t n = strlen(s);
    uint8_t sign = MPD_POS;
    size_t pos = 2;

    if (s[0] == '-')
    {
        sign = MPD_NEG;
        pos = 3;
    }

    int digits = n - pos;
    uint16_t rdata[digits];

    for (int i = 0; i < digits; i++)
    {
        int c = s[pos+i] & 0xff;
        if (isdigit(c))
        {
            c -= '0';
        } else if (isalpha(c)) {
            c -= isupper(c) ? 'A' - 10 : 'a' - 10;
        }

        rdata[digits - i - 1] = c;
    }

    uint32_t status = 0;
    mpd_t *r = mpd_qnew();
    mpd_qimport_u16(r, &rdata[0], digits, sign, 16, mpd_max_ctx(), &status);
    return opa_bf_to_number(r);
}

WASM_EXPORT(test_bits)
void test_bits(void)
{
    // tests from https://golang.org/src/math/big/int_test.go L1193

    struct and_or_xor_test
    {
        const char *x;
        const char *y;
        const char *and;
        const char *or;
        const char *xor;
    };

    struct and_or_xor_test tests1[] = {
        {"0x00", "0x00", "0x00", "0x00", "0x00"},
        {"0x00", "0x01", "0x00", "0x01", "0x01"},
        {"0x01", "0x00", "0x00", "0x01", "0x01"},
        {"-0x01", "0x00", "0x00", "-0x01", "-0x01"},
        {"-0xaf", "-0x50", "-0xf0", "-0x0f", "0xe1"},
        {"0x00", "-0x01", "0x00", "-0x01", "-0x01"},
        {"0x01", "0x01", "0x01", "0x01", "0x00"},
        {"-0x01", "-0x01", "-0x01", "-0x01", "0x00"},
        {"0x07", "0x08", "0x00", "0x0f", "0x0f"},
        {"0x05", "0x0f", "0x05", "0x0f", "0x0a"},
        {"0xff", "-0x0a", "0xf6", "-0x01", "-0xf7"},
        {"0x013ff6", "0x9a4e", "0x1a46", "0x01bffe", "0x01a5b8"},
        {"-0x013ff6", "0x9a4e", "0x800a", "-0x0125b2", "-0x01a5bc"},
        {"-0x013ff6", "-0x9a4e", "-0x01bffe", "-0x1a46", "0x01a5b8"},
        {
            "0x1000009dc6e3d9822cba04129bcbe3401",
            "0xb9bd7d543685789d57cb918e833af352559021483cdb05cc21fd",
            "0x1000001186210100001000009048c2001",
            "0xb9bd7d543685789d57cb918e8bfeff7fddb2ebe87dfbbdfe35fd",
            "0xb9bd7d543685789d57ca918e8ae69d6fcdb2eae87df2b97215fc",
            },
        {
            "0x1000009dc6e3d9822cba04129bcbe3401",
            "-0xb9bd7d543685789d57cb918e833af352559021483cdb05cc21fd",
            "0x8c40c2d8822caa04120b8321401",
            "-0xb9bd7d543685789d57ca918e82229142459020483cd2014001fd",
            "-0xb9bd7d543685789d57ca918e8ae69d6fcdb2eae87df2b97215fe",
        },
        {
            "-0x1000009dc6e3d9822cba04129bcbe3401",
            "-0xb9bd7d543685789d57cb918e833af352559021483cdb05cc21fd",
            "-0xb9bd7d543685789d57cb918e8bfeff7fddb2ebe87dfbbdfe35fd",
            "-0x1000001186210100001000009048c2001",
            "0xb9bd7d543685789d57ca918e8ae69d6fcdb2eae87df2b97215fc",
        },
    };

    for (int i = 0; i < sizeof(tests1)/sizeof(tests1[0]); i++) {
        test("and", opa_value_compare(number(tests1[i].and), opa_bits_and(number(tests1[i].x), number(tests1[i].y))) == 0);
        test("or", opa_value_compare(number(tests1[i].or), opa_bits_or(number(tests1[i].x), number(tests1[i].y))) == 0);
        test("xor", opa_value_compare(number(tests1[i].xor), opa_bits_xor(number(tests1[i].x), number(tests1[i].y))) == 0);
    }

    // tests from https://golang.org/src/math/big/int_test.go L1496

    struct negate_test
    {
        const char *input;
        const char *output;
    };

    struct negate_test tests2[] = {
        {"0", "-1"},
        {"1", "-2"},
        {"7", "-8"},
        {"0", "-1"},
        {"-81910", "81909"},
        {
            "298472983472983471903246121093472394872319615612417471234712061",
            "-298472983472983471903246121093472394872319615612417471234712062",
        },
     };

     for (int i = 0; i < sizeof(tests2)/sizeof(tests2[0]); i++) {
         test("negate", opa_value_compare(opa_number_ref(tests2[i].output, strlen(tests2[i].output)),
                                          opa_bits_negate(opa_number_ref(tests2[i].input, strlen(tests2[i].input)))) == 0);
         test("negate", opa_value_compare(opa_number_ref(tests2[i].input, strlen(tests2[i].input)),
                                          opa_bits_negate(opa_number_ref(tests2[i].output, strlen(tests2[i].output)))) == 0);
     }

     // tests from https://golang.org/src/math/big/int_test.go L883

     struct shift_test
     {
         const char *input;
         int shift;
         const char *output;
     };

     struct shift_test tests3[] = {
         {"0", 0, "0"},
         {"-0", 0, "0"},
         {"0", 1, "0"},
         {"0", 2, "0"},
         {"1", 0, "1"},
         {"1", 1, "0"},
         {"1", 2, "0"},
         {"2", 0, "2"},
         {"2", 1, "1"},
         {"-1", 0, "-1"},
         {"-1", 1, "-1"},
         {"-1", 10, "-1"},
         {"-100", 2, "-25"},
         {"-100", 3, "-13"},
         {"-100", 100, "-1"},
         {"4294967296", 0, "4294967296"},
         {"4294967296", 1, "2147483648"},
         {"4294967296", 2, "1073741824"},
         {"18446744073709551616", 0, "18446744073709551616"},
         {"18446744073709551616", 1, "9223372036854775808"},
         {"18446744073709551616", 2, "4611686018427387904"},
         {"18446744073709551616", 64, "1"},
         {"340282366920938463463374607431768211456", 64, "18446744073709551616"},
         {"340282366920938463463374607431768211456", 128, "1"},
     };

     for (int i = 0; i < sizeof(tests3)/sizeof(tests3[0]); i++) {
         test("right shift", opa_value_compare(opa_number_ref(tests3[i].output, strlen(tests3[i].output)),
                                               opa_bits_shiftright(opa_number_ref(tests3[i].input, strlen(tests3[i].input)),
                                                                   opa_number_int(tests3[i].shift))) == 0);
     };

     // tests from https://golang.org/src/math/big/int_test.go L940

     struct shift_test tests4[] = {
         {"0", 0, "0"},
         {"0", 1, "0"},
         {"0", 2, "0"},
         {"1", 0, "1"},
         {"1", 1, "2"},
         {"1", 2, "4"},
         {"2", 0, "2"},
         {"2", 1, "4"},
         {"2", 2, "8"},
         {"-87", 1, "-174"},
         {"4294967296", 0, "4294967296"},
         {"4294967296", 1, "8589934592"},
         {"4294967296", 2, "17179869184"},
         {"18446744073709551616", 0, "18446744073709551616"},
         {"9223372036854775808", 1, "18446744073709551616"},
         {"4611686018427387904", 2, "18446744073709551616"},
         {"1", 64, "18446744073709551616"},
         {"18446744073709551616", 64, "340282366920938463463374607431768211456"},
         {"1", 128, "340282366920938463463374607431768211456"},
     };

     for (int i = 0; i < sizeof(tests4)/sizeof(tests4[0]); i++) {
         test("left shift", opa_value_compare(opa_number_ref(tests4[i].output, strlen(tests4[i].output)),
                                              opa_bits_shiftleft(opa_number_ref(tests4[i].input, strlen(tests4[i].input)),
                                                                 opa_number_int(tests4[i].shift))) == 0);
     };
}

WASM_EXPORT(test_aggregates)
void test_aggregates(void)
{
    opa_array_t *arr = opa_cast_array(opa_array());
    opa_array_append(arr, opa_number_int(2));
    opa_array_append(arr, opa_number_int(1));
    opa_array_append(arr, opa_number_int(4));

    opa_array_t *arr_sorted = opa_cast_array(opa_array());
    opa_array_append(arr_sorted, opa_number_int(1));
    opa_array_append(arr_sorted, opa_number_int(2));
    opa_array_append(arr_sorted, opa_number_int(4));

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj, opa_string_terminated("c"), opa_number_int(4));

    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_number_int(2));
    opa_set_add(set, opa_number_int(1));
    opa_set_add(set, opa_number_int(4));

    test("count/string", opa_value_compare(opa_agg_count(opa_string("foo", 3)), opa_number_int(3)) == 0);
    test("count/unicode string", opa_value_compare(opa_agg_count(opa_string("\xC3\xA5\xC3\xA4\xC3\xB6", 6)), opa_number_int(3)) == 0);
    test("count/array", opa_value_compare(opa_agg_count(&arr->hdr), opa_number_int(3)) == 0);
    test("count/object", opa_value_compare(opa_agg_count(&obj->hdr), opa_number_int(3)) == 0);
    test("count/set", opa_value_compare(opa_agg_count(&set->hdr), opa_number_int(3)) == 0);

    test("sum/array", opa_value_compare(opa_agg_sum(&arr->hdr), opa_number_int(7)) == 0);
    test("sum/set", opa_value_compare(opa_agg_sum(&set->hdr), opa_number_int(7)) == 0);

    test("product/array", opa_value_compare(opa_agg_product(&arr->hdr), opa_number_int(8)) == 0);
    test("product/set", opa_value_compare(opa_agg_product(&set->hdr), opa_number_int(8)) == 0);

    test("max/array", opa_value_compare(opa_agg_max(&arr->hdr), opa_number_int(4)) == 0);
    test("max/set", opa_value_compare(opa_agg_max(&set->hdr), opa_number_int(4)) == 0);

    test("min/array", opa_value_compare(opa_agg_min(&arr->hdr), opa_number_int(1)) == 0);
    test("min/set", opa_value_compare(opa_agg_min(&set->hdr), opa_number_int(1)) == 0);

    test("sort/array", opa_value_compare(opa_agg_sort(&arr->hdr), &arr_sorted->hdr) == 0);
    test("sort/set", opa_value_compare(opa_agg_sort(&set->hdr), &arr_sorted->hdr) == 0);

    opa_array_t *arr_trues = opa_cast_array(opa_array());
    opa_array_append(arr_trues, opa_boolean(true));
    opa_array_append(arr_trues, opa_boolean(true));

    opa_array_t *arr_mixed = opa_cast_array(opa_array());
    opa_array_append(arr_mixed, opa_boolean(true));
    opa_array_append(arr_mixed, opa_boolean(false));

    opa_array_t *arr_falses = opa_cast_array(opa_array());
    opa_array_append(arr_falses, opa_boolean(false));
    opa_array_append(arr_falses, opa_boolean(false));

    test("all/array trues", opa_value_compare(opa_agg_all(&arr_trues->hdr), opa_boolean(true)) == 0);
    test("all/array mixed", opa_value_compare(opa_agg_all(&arr_mixed->hdr), opa_boolean(false)) == 0);
    test("all/array falses", opa_value_compare(opa_agg_all(&arr_falses->hdr), opa_boolean(false)) == 0);
    test("any/array trues", opa_value_compare(opa_agg_any(&arr_trues->hdr), opa_boolean(true)) == 0);
    test("any/array mixed", opa_value_compare(opa_agg_any(&arr_mixed->hdr), opa_boolean(true)) == 0);
    test("any/array falses", opa_value_compare(opa_agg_any(&arr_falses->hdr), opa_boolean(false)) == 0);

    opa_set_t *set_trues = opa_cast_set(opa_set());
    opa_set_add(set_trues, opa_boolean(true));
    opa_set_add(set_trues, opa_boolean(true));

    opa_set_t *set_mixed = opa_cast_set(opa_set());
    opa_set_add(set_mixed, opa_boolean(true));
    opa_set_add(set_mixed, opa_boolean(false));

    opa_set_t *set_falses = opa_cast_set(opa_set());
    opa_set_add(set_falses, opa_boolean(false));
    opa_set_add(set_falses, opa_boolean(false));

    test("all/set trues", opa_value_compare(opa_agg_all(&set_trues->hdr), opa_boolean(true)) == 0);
    test("all/set mixed", opa_value_compare(opa_agg_all(&set_mixed->hdr), opa_boolean(false)) == 0);
    test("all/set falses", opa_value_compare(opa_agg_all(&set_falses->hdr), opa_boolean(false)) == 0);
    test("any/set trues", opa_value_compare(opa_agg_any(&set_trues->hdr), opa_boolean(true)) == 0);
    test("any/set mixed", opa_value_compare(opa_agg_any(&set_mixed->hdr), opa_boolean(true)) == 0);
    test("any/set falses", opa_value_compare(opa_agg_any(&set_falses->hdr), opa_boolean(false)) == 0);
}

WASM_EXPORT(test_base64)
void test_base64(void)
{
    test("base64/is_valid", opa_value_compare(opa_base64_is_valid(opa_string_terminated("YWJjMTIzIT8kKiYoKSctPUB+")), opa_boolean(true)) == 0);
    test("base64/encode", opa_value_compare(opa_base64_encode(opa_string_terminated("abc123!?$*&()'-=@~")), opa_string_terminated("YWJjMTIzIT8kKiYoKSctPUB+")) == 0);
    test("base64/encode", opa_value_compare(opa_base64_encode(opa_string_terminated("This is a long string that should not be split to many lines")),
                                            opa_string_terminated("VGhpcyBpcyBhIGxvbmcgc3RyaW5nIHRoYXQgc2hvdWxkIG5vdCBiZSBzcGxpdCB0byBtYW55IGxpbmVz")) == 0);
    test("base64/decode", opa_value_compare(opa_base64_decode(opa_string_terminated("YWJjMTIzIT8kKiYoKSctPUB+")), opa_string_terminated("abc123!?$*&()'-=@~")) == 0);
    test("base64/decode", opa_value_compare(opa_base64_decode(opa_string_terminated("VGhpcyBpcyBhIGxvbmcgc3RyaW5nIHRoYXQgc2hvdWxkIG5vdCBiZSBzcGxpdCB0byBtYW55IGxpbmVz")),
                                            opa_string_terminated("This is a long string that should not be split to many lines")) == 0);
    test("base64/decode", opa_value_compare(opa_base64_decode(opa_string_terminated("VGhpcyBpcyBhIGxvbmcgc3RyaW5nIHRoYXQgY2FuIGJlIHBhcnNlZCBldmVuIGlmIHNwbGl0IHRv\nIG1hbnkgbGluZXM=")),
                                            opa_string_terminated("This is a long string that can be parsed even if split to many lines")) == 0);
    test("base64/url_encode", opa_value_compare(opa_base64_url_encode(opa_string_terminated("abc123!?$*&()'-=@~")), opa_string_terminated("YWJjMTIzIT8kKiYoKSctPUB-")) == 0);
    test("base64/url_decode", opa_value_compare(opa_base64_url_decode(opa_string_terminated("YWJjMTIzIT8kKiYoKSctPUB-")), opa_string_terminated("abc123!?$*&()'-=@~")) == 0);
}

WASM_EXPORT(test_json)
void test_json(void)
{
    test("json/marshal", opa_value_compare(opa_json_marshal(opa_string_terminated("string")), opa_string_terminated("\"string\"")) == 0);
    test("json/unmarshal", opa_value_compare(opa_json_unmarshal(opa_string_terminated("\"string\"")), opa_string_terminated("string")) == 0);
    test("json/is_valid_true", opa_cast_boolean(opa_json_is_valid(opa_string_terminated("\"string\"")))->v);
    test("json/is_valid_false", !opa_cast_boolean(opa_json_is_valid(opa_string_terminated("\"string")))->v);
}

WASM_EXPORT(test_object)
void test_object(void)
{
    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(obj, opa_string_terminated("c"), opa_number_int(3));

    test("object/get (key found)", opa_value_compare(builtin_object_get(&obj->hdr, opa_string_terminated("a"), opa_number_int(2)), opa_number_int(1)) == 0);
    test("object/get (string key not found)", opa_value_compare(builtin_object_get(&obj->hdr, opa_string_terminated("d"), opa_number_int(2)), opa_number_int(2)) == 0);
    test("object/get (integer key not found)", opa_value_compare(builtin_object_get(&obj->hdr, opa_number_int(1), opa_number_int(2)), opa_number_int(2)) == 0);
    test("object/get (boolean default value)", opa_value_compare(builtin_object_get(&obj->hdr, opa_number_int(1), opa_boolean(true)), opa_boolean(true)) == 0);
    test("object/get (non-object operand)", opa_value_compare(builtin_object_get(opa_string_terminated("a"), opa_number_int(1), opa_boolean(true)), NULL) == 0);

    opa_object_t *obj_keys = opa_cast_object(opa_object());
    opa_object_insert(obj_keys, opa_string_terminated("a"), opa_number_int(0));
    opa_object_insert(obj_keys, opa_string_terminated("c"), opa_number_int(0));

    opa_object_t *expected = opa_cast_object(opa_object());
    opa_object_insert(expected, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(expected, opa_string_terminated("c"), opa_number_int(3));
    test("object/filter (object keys)", opa_value_compare(builtin_object_filter(&obj->hdr, &obj_keys->hdr), &expected->hdr) == 0);

    opa_set_t *set_keys = opa_cast_set(opa_set());
    opa_set_add(set_keys, opa_string_terminated("a"));
    opa_set_add(set_keys, opa_string_terminated("c"));
    test("object/filter (set keys)", opa_value_compare(builtin_object_filter(&obj->hdr, &set_keys->hdr), &expected->hdr) == 0);

    opa_array_t *arr_keys = opa_cast_array(opa_array());
    opa_array_append(arr_keys, opa_string_terminated("a"));
    opa_array_append(arr_keys, opa_string_terminated("c"));
    test("object/filter (array keys)", opa_value_compare(builtin_object_filter(&obj->hdr, &arr_keys->hdr), &expected->hdr) == 0);
}

WASM_EXPORT(test_object_keys)
void test_object_keys(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj1, opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(3));

    opa_set_t *expected_keys1 = opa_cast_set(opa_set());
    opa_set_add(expected_keys1, opa_string_terminated("a"));
    opa_set_add(expected_keys1, opa_string_terminated("b"));
    opa_set_add(expected_keys1, opa_string_terminated("c"));

    test("object/keys (string keys)", opa_value_compare(builtin_object_keys(&obj1->hdr), &expected_keys1->hdr) == 0);

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_number_int(1), opa_number_int(2));
    opa_object_insert(obj2, opa_number_int(3), opa_number_int(4));

    opa_set_t *expected_keys2 = opa_cast_set(opa_set());
    opa_set_add(expected_keys2, opa_number_int(1));
    opa_set_add(expected_keys2, opa_number_int(3));

    test("object/keys (number keys)", opa_value_compare(builtin_object_keys(&obj2->hdr), &expected_keys2->hdr) == 0);

    opa_set_t *set_key = opa_cast_set(opa_set());
    opa_set_add(set_key, opa_number_int(1));
    opa_set_add(set_key, opa_number_int(2));

    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, &set_key->hdr, opa_number_int(1));

    opa_set_t *expected_keys3 = opa_cast_set(opa_set());
    opa_set_add(expected_keys3, &set_key->hdr);

    test("object/keys (set keys)", opa_value_compare(builtin_object_keys(&obj3->hdr), &expected_keys3->hdr) == 0);

    opa_object_t *object_key = opa_cast_object(opa_object());
    opa_object_insert(object_key, opa_string_terminated("a"), opa_number_int(1));

    opa_object_t *obj4 = opa_cast_object(opa_object());
    opa_object_insert(obj4, &object_key->hdr, opa_number_int(1));

    opa_set_t *expected_keys4 = opa_cast_set(opa_set());
    opa_set_add(expected_keys4, &object_key->hdr);

    test("object/keys (object keys)", opa_value_compare(builtin_object_keys(&obj4->hdr), &expected_keys4->hdr) == 0);

    opa_array_t *array_key = opa_cast_array(opa_array());
    opa_array_append(array_key, opa_number_int(1));
    opa_array_append(array_key, opa_number_int(2));

    opa_object_t *obj5 = opa_cast_object(opa_object());
    opa_object_insert(obj5, &array_key->hdr, opa_number_int(1));

    opa_set_t *expected_keys5 = opa_cast_set(opa_set());
    opa_set_add(expected_keys5, &array_key->hdr);

    test("object/keys (array keys)", opa_value_compare(builtin_object_keys(&obj5->hdr), &expected_keys5->hdr) == 0);

    opa_object_t *obj6 = opa_cast_object(opa_object());
    opa_set_t *expected_keys6 = opa_cast_set(opa_set());
    test("object/keys (empty)", opa_value_compare(builtin_object_keys(&obj6->hdr), &expected_keys6->hdr) == 0);

    test("object/keys (null on non-object)", opa_value_compare(builtin_object_keys(opa_number_int(3)), NULL) == 0);
}

WASM_EXPORT(test_object_remove)
void test_object_remove(void)
{
    opa_object_t *o = opa_cast_object(opa_object());
    opa_object_insert(o, opa_string_terminated("c"), opa_number_int(3));

    // input -> {"a": 1, "b": {"c": 3}}
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj1, opa_string_terminated("b"), &o->hdr);

    opa_set_t *set_keys1 = opa_cast_set(opa_set());
    opa_set_add(set_keys1, opa_string_terminated("a"));

    opa_object_t *expected1 = opa_cast_object(opa_object());
    opa_object_insert(expected1, opa_string_terminated("b"), &o->hdr);
    test("object/remove (base)", opa_value_compare(builtin_object_remove(&obj1->hdr, &set_keys1->hdr), &expected1->hdr) == 0);

    // input -> {"a": 1, "b": {"c": 3}, "d": 4}
    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj2, opa_string_terminated("b"), &o->hdr);
    opa_object_insert(obj2, opa_string_terminated("d"), opa_number_int(4));

    opa_set_t *set_keys2 = opa_cast_set(opa_set());
    opa_set_add(set_keys2, opa_string_terminated("b"));
    opa_set_add(set_keys2, opa_string_terminated("d"));

    opa_object_t *expected2 = opa_cast_object(opa_object());
    opa_object_insert(expected2, opa_string_terminated("a"), opa_number_int(1));
    test("object/remove (multiple keys set)", opa_value_compare(builtin_object_remove(&obj2->hdr, &set_keys2->hdr), &expected2->hdr) == 0);

    opa_array_t *arr_keys = opa_cast_array(opa_array());
    opa_array_append(arr_keys, opa_string_terminated("b"));
    opa_array_append(arr_keys, opa_string_terminated("d"));
    test("object/remove (multiple keys array)", opa_value_compare(builtin_object_remove(&obj2->hdr, &arr_keys->hdr), &expected2->hdr) == 0);

    opa_object_t *obj_keys = opa_cast_object(opa_object());
    opa_object_insert(obj_keys, opa_string_terminated("b"), opa_number_int(1));
    opa_object_insert(obj_keys, opa_string_terminated("d"), opa_string_terminated(""));
    test("object/remove (multiple keys object)", opa_value_compare(builtin_object_remove(&obj2->hdr, &obj_keys->hdr), &expected2->hdr) == 0);

    // input -> {"a": {"b": {"c": 3}}, "x": 123}
    opa_object_t *o2 = opa_cast_object(opa_object());
    opa_object_insert(o2, opa_string_terminated("b"), &o->hdr);

    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, opa_string_terminated("a"), &o2->hdr);
    opa_object_insert(obj3, opa_string_terminated("x"), opa_number_int(123));

    opa_object_t *o_keys1 = opa_cast_object(opa_object());
    opa_object_insert(o_keys1, opa_string_terminated("foo"), opa_string_terminated("bar"));

    opa_object_t *o_keys2 = opa_cast_object(opa_object());
    opa_object_insert(o_keys2, opa_string_terminated("b"), &o_keys1->hdr);

    opa_object_t *obj_keys2 = opa_cast_object(opa_object());
    opa_object_insert(obj_keys2, opa_string_terminated("a"), &o_keys2->hdr);

    opa_object_t *expected3 = opa_cast_object(opa_object());
    opa_object_insert(expected3, opa_string_terminated("x"), opa_number_int(123));
    test("object/remove (multiple keys object nested)", opa_value_compare(builtin_object_remove(&obj3->hdr, &obj_keys2->hdr), &expected3->hdr) == 0);

    test("object/remove (empty object)", opa_value_compare(builtin_object_remove(opa_object(), &obj_keys2->hdr), opa_object()) == 0);

    test("object/remove (empty keys set)", opa_value_compare(builtin_object_remove(&obj3->hdr, opa_set()), &obj3->hdr) == 0);

    test("object/remove (empty keys array)", opa_value_compare(builtin_object_remove(&obj3->hdr, opa_array()), &obj3->hdr) == 0);

    test("object/remove (empty keys object)", opa_value_compare(builtin_object_remove(&obj3->hdr, opa_object()), &obj3->hdr) == 0);

    test("object/remove (non-object first operand)", opa_value_compare(builtin_object_remove(opa_string_terminated("a"),  opa_object()), NULL) == 0);

    opa_set_t *set_keys3 = opa_cast_set(opa_set());
    opa_set_add(set_keys3, opa_string_terminated("foo"));
    test("object/remove (key does not exist)", opa_value_compare(builtin_object_remove(&obj3->hdr, &set_keys3->hdr), &obj3->hdr) == 0);

    test("object/remove (second operand not object/set/array)", opa_value_compare(builtin_object_remove(&obj3->hdr, opa_string_terminated("a")), NULL) == 0);
}

WASM_EXPORT(test_object_union)
void test_object_union(void)
{
    test("object/union (both empty)", opa_value_compare(builtin_object_union(opa_object(), opa_object()), opa_object()) == 0);

    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("a"), opa_number_int(1));
    test("object/union (left empty)", opa_value_compare(builtin_object_union(opa_object(), &obj1->hdr), &obj1->hdr) == 0);

    test("object/union (right empty)", opa_value_compare(builtin_object_union(&obj1->hdr, opa_object()), &obj1->hdr) == 0);

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("b"), opa_number_int(2));

    opa_object_t *expected1 = opa_cast_object(opa_object());
    opa_object_insert(expected1, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(expected1, opa_string_terminated("b"), opa_number_int(2));

    test("object/union (base)", opa_value_compare(builtin_object_union(&obj1->hdr, &obj2->hdr), &expected1->hdr) == 0);

    opa_object_t *o = opa_cast_object(opa_object());
    opa_object_insert(o, opa_string_terminated("c"), opa_number_int(3));
    opa_object_t *o2 = opa_cast_object(opa_object());
    opa_object_insert(o2, opa_string_terminated("b"), &o->hdr);
    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, opa_string_terminated("a"), &o2->hdr);

    opa_object_t *expected2 = opa_cast_object(opa_object());
    opa_object_insert(expected2, opa_string_terminated("a"),&o2->hdr);
    opa_object_insert(expected2, opa_string_terminated("b"), opa_number_int(2));

    test("object/union (nested)", opa_value_compare(builtin_object_union(&obj3->hdr, &obj2->hdr), &expected2->hdr) == 0);
    test("object/union (nested reverse)", opa_value_compare(builtin_object_union(&obj2->hdr, &obj3->hdr), &expected2->hdr) == 0);

    opa_object_t *obj4 = opa_cast_object(opa_object());
    opa_object_insert(obj4, opa_string_terminated("a"), opa_number_int(2));

    test("object/union (conflict simple)", opa_value_compare(builtin_object_union(&obj1->hdr, &obj4->hdr), &obj4->hdr) == 0);

    opa_object_insert(obj3, opa_string_terminated("d"), opa_number_int(7));

    test("object/union (conflict nested and extra field)", opa_value_compare(builtin_object_union(&obj1->hdr, &obj3->hdr), &obj3->hdr) == 0);

    // Operand 1 -> {"a": {"b": {"c": 1}}, "e": 1}
    opa_object_t *o3 = opa_cast_object(opa_object());
    opa_object_insert(o3, opa_string_terminated("c"), opa_number_int(1));
    opa_object_t *o4 = opa_cast_object(opa_object());
    opa_object_insert(o4, opa_string_terminated("b"), &o3->hdr);
    opa_object_t *obj5 = opa_cast_object(opa_object());
    opa_object_insert(obj5, opa_string_terminated("a"), &o4->hdr);
    opa_object_insert(obj5, opa_string_terminated("e"), opa_number_int(1));

    // Operand 2 -> {"a": {"b": "foo", "b1": "bar"}, "d": 7, "e": 17}
    opa_object_t *o5 = opa_cast_object(opa_object());
    opa_object_insert(o5, opa_string_terminated("b"), opa_string_terminated("foo"));
    opa_object_insert(o5, opa_string_terminated("b1"), opa_string_terminated("bar"));
    opa_object_t *obj6 = opa_cast_object(opa_object());
    opa_object_insert(obj6, opa_string_terminated("a"), &o5->hdr);
    opa_object_insert(obj6, opa_string_terminated("d"), opa_number_int(7));
    opa_object_insert(obj6, opa_string_terminated("e"), opa_number_int(17));

    // Expected -> {"a": {"b": "foo", "b1": "bar"}, "d": 7, "e": 17}
    opa_object_t *o6 = opa_cast_object(opa_object());
    opa_object_insert(o6, opa_string_terminated("b"), opa_string_terminated("foo"));
    opa_object_insert(o6, opa_string_terminated("b1"), opa_string_terminated("bar"));
    opa_object_t *expected3 = opa_cast_object(opa_object());
    opa_object_insert(expected3, opa_string_terminated("a"), &o6->hdr);
    opa_object_insert(expected3, opa_string_terminated("d"), opa_number_int(7));
    opa_object_insert(expected3, opa_string_terminated("e"), opa_number_int(17));

    test("object/union (conflict multiple-1)", opa_value_compare(builtin_object_union(&obj5->hdr, &obj6->hdr), &expected3->hdr) == 0);

    // Operand 1 -> {"a": {"b": {"c": 1, "d": 2}}, "e": 1}
    opa_object_t *o7 = opa_cast_object(opa_object());
    opa_object_insert(o7, opa_string_terminated("c"), opa_number_int(1));
    opa_object_insert(o7, opa_string_terminated("d"), opa_number_int(2));
    opa_object_t *o8 = opa_cast_object(opa_object());
    opa_object_insert(o8, opa_string_terminated("b"), &o7->hdr);
    opa_object_t *obj7 = opa_cast_object(opa_object());
    opa_object_insert(obj7, opa_string_terminated("a"), &o8->hdr);
    opa_object_insert(obj7, opa_string_terminated("e"), opa_number_int(1));

    // Operand 2 -> {"a": {"b": {"c": "foo"}, "b1": "bar"}, "d": 7, "e": 17}
    opa_object_t *o9 = opa_cast_object(opa_object());
    opa_object_insert(o9, opa_string_terminated("c"), opa_string_terminated("foo"));
    opa_object_t *o10 = opa_cast_object(opa_object());
    opa_object_insert(o10, opa_string_terminated("b"), &o9->hdr);
    opa_object_insert(o10, opa_string_terminated("b1"), opa_string_terminated("bar"));
    opa_object_t *obj8 = opa_cast_object(opa_object());
    opa_object_insert(obj8, opa_string_terminated("a"), &o10->hdr);
    opa_object_insert(obj8, opa_string_terminated("d"), opa_number_int(7));
    opa_object_insert(obj8, opa_string_terminated("e"), opa_number_int(17));

    // Expected -> {"a": {"b": {"c": "foo", "d": 2}, "b1": "bar"}, "d": 7, "e": 17}
    opa_object_t *o11 = opa_cast_object(opa_object());
    opa_object_insert(o11, opa_string_terminated("c"), opa_string_terminated("foo"));
    opa_object_insert(o11, opa_string_terminated("d"), opa_number_int(2));
    opa_object_t *o12 = opa_cast_object(opa_object());
    opa_object_insert(o12, opa_string_terminated("b"), &o11->hdr);
    opa_object_insert(o12, opa_string_terminated("b1"), opa_string_terminated("bar"));
    opa_object_t *expected4 = opa_cast_object(opa_object());
    opa_object_insert(expected4, opa_string_terminated("a"), &o12->hdr);
    opa_object_insert(expected4, opa_string_terminated("d"), opa_number_int(7));
    opa_object_insert(expected4, opa_string_terminated("e"), opa_number_int(17));

    test("object/union (conflict multiple-2)", opa_value_compare(builtin_object_union(&obj7->hdr, &obj8->hdr), &expected4->hdr) == 0);

    opa_object_t *obj9 = opa_cast_object(opa_object());
    opa_object_insert(obj9, opa_string_terminated("a"), opa_string_terminated("foo"));
    opa_object_insert(obj9, opa_string_terminated("b"), opa_string_terminated("bar"));

    opa_object_t *obj10 = opa_cast_object(opa_object());
    opa_object_insert(obj10, opa_string_terminated("a"), opa_string_terminated("baz"));

    opa_object_t *expected5 = opa_cast_object(opa_object());
    opa_object_insert(expected5, opa_string_terminated("a"), opa_string_terminated("baz"));
    opa_object_insert(expected5, opa_string_terminated("b"), opa_string_terminated("bar"));

    test("object/union (conflict multiple-3)", opa_value_compare(builtin_object_union(&obj9->hdr, &obj10->hdr), &expected5->hdr) == 0);

    test("object/union (non-object first operand)", opa_value_compare(builtin_object_union(opa_string_terminated("a"),  opa_object()), NULL) == 0);

    test("object/union (non-object second operand)", opa_value_compare(builtin_object_union(opa_object(), opa_string_terminated("a")), NULL) == 0);
}

opa_object_t *json_test_fixture_object1(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(7));
    opa_object_insert(obj1, opa_string_terminated("d"), opa_number_int(8));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("b"), &obj1->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj2->hdr);
    opa_object_insert(obj, opa_string_terminated("e"), opa_number_int(9));
    return obj;
}

opa_object_t *json_test_fixture_object2(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(7));
    opa_object_insert(obj1, opa_string_terminated("d"), opa_number_int(8));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("b"), &obj1->hdr);
    opa_object_insert(obj2, opa_string_terminated("e"), opa_number_int(9));

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj2->hdr);

    return obj;
}

opa_object_t *json_test_fixture_object3(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("b"), opa_number_int(7));

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj1->hdr);
    opa_object_insert(obj, opa_string_terminated("c"), opa_number_int(1));

    return obj;
}

opa_object_t *json_test_fixture_object4(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("b"), opa_number_int(7));
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(8));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("d"), opa_number_int(9));

    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, &obj1->hdr);
    opa_array_append(arr1, &obj2->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &arr1->hdr);

    return obj;
}

opa_object_t *json_test_fixture_object5(void)
{
    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_string_terminated("b"));
    opa_array_append(arr1, opa_string_terminated("c"));
    opa_array_append(arr1, opa_string_terminated("d"));

    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("1"), &arr1->hdr);

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("x"), opa_string_terminated("y"));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, &obj1->hdr);
    opa_array_append(arr2, &obj2->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &arr2->hdr);

    return obj;
}

opa_object_t *json_test_fixture_object6(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("c"), opa_number_int(7));
    opa_object_insert(obj1, opa_string_terminated("d"), opa_number_int(8));
    opa_object_insert(obj1, opa_string_terminated("x"), opa_number_int(0));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("b"), &obj1->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj2->hdr);
    opa_object_insert(obj, opa_string_terminated("e"), opa_number_int(9));
    return obj;
}

opa_object_t *json_remove_get_exp_object1(void)
{
    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_string_terminated("b"));
    opa_array_append(arr1, opa_string_terminated("c"));

    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("1"), &arr1->hdr);

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("x"), opa_string_terminated("y"));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, &obj1->hdr);
    opa_array_append(arr2, &obj2->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &arr2->hdr);

    return obj;
}

opa_object_t *json_remove_get_exp_object2(void)
{
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("x"), opa_number_int(0));

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("b"), &obj1->hdr);

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), &obj2->hdr);
    opa_object_insert(obj, opa_string_terminated("e"), opa_number_int(9));
    return obj;
}

WASM_EXPORT(test_json_remove)
void test_json_remove(void)
{
    opa_object_t *obj1 = json_test_fixture_object1();

    opa_set_t *set_paths1 = opa_cast_set(opa_set());
    opa_set_add(set_paths1, opa_string_terminated("a/b/c"));

    opa_object_t *o1 = opa_cast_object(opa_object());
    opa_object_insert(o1, opa_string_terminated("d"), opa_number_int(8));
    opa_object_t *o2 = opa_cast_object(opa_object());
    opa_object_insert(o2, opa_string_terminated("b"), &o1->hdr);

    opa_object_t *expected1 = opa_cast_object(opa_object());
    opa_object_insert(expected1, opa_string_terminated("a"), &o2->hdr);
    opa_object_insert(expected1, opa_string_terminated("e"), opa_number_int(9));

    test("jsonremove/base", opa_value_compare(builtin_json_remove(&obj1->hdr, &set_paths1->hdr), &expected1->hdr) == 0);

    opa_set_add(set_paths1, opa_string_terminated("e"));

    opa_object_t *expected2 = opa_cast_object(opa_object());
    opa_object_insert(expected2, opa_string_terminated("a"), &o2->hdr);

    test("jsonremove/multiple roots", opa_value_compare(builtin_json_remove(&obj1->hdr, &set_paths1->hdr), &expected2->hdr) == 0);

    opa_array_t *array_paths1 = opa_cast_array(opa_array());
    opa_array_append(array_paths1, opa_string_terminated("a/b/c"));
    opa_array_append(array_paths1, opa_string_terminated("e"));

    test("jsonremove/multiple roots array", opa_value_compare(builtin_json_remove(&obj1->hdr, &array_paths1->hdr), &expected2->hdr) == 0);

    opa_object_t *obj2 = json_test_fixture_object2();

    opa_set_t *set_paths2 = opa_cast_set(opa_set());
    opa_set_add(set_paths2, opa_string_terminated("a/b/c"));
    opa_set_add(set_paths2, opa_string_terminated("a/e"));

    test("jsonremove/shared roots", opa_value_compare(builtin_json_remove(&obj2->hdr, &set_paths2->hdr), &expected2->hdr) == 0);

    opa_object_t *obj3 = json_test_fixture_object3();
    opa_set_t *set_paths3 = opa_cast_set(opa_set());
    opa_set_add(set_paths3, opa_string_terminated("a"));
    opa_set_add(set_paths3, opa_string_terminated("a/b"));

    opa_object_t *expected3 = opa_cast_object(opa_object());
    opa_object_insert(expected3, opa_string_terminated("c"), opa_number_int(1));

    test("jsonremove/conflict", opa_value_compare(builtin_json_remove(&obj3->hdr, &set_paths3->hdr), &expected3->hdr) == 0);

    opa_object_t *obj4 = opa_cast_object(opa_object());
    opa_object_insert(obj4, opa_string_terminated("a"), opa_number_int(7));

    test("jsonremove/empty list", opa_value_compare(builtin_json_remove(&obj4->hdr, opa_set()), &obj4->hdr) == 0);

    test("jsonremove/empty object", opa_value_compare(builtin_json_remove(opa_object(), &set_paths3->hdr), opa_object()) == 0);

    opa_object_t *obj5 = json_test_fixture_object1();

    opa_set_t *set_paths4 = opa_cast_set(opa_set());
    opa_set_add(set_paths4, opa_string_terminated("a"));
    opa_set_add(set_paths4, opa_string_terminated("e"));

    test("jsonremove/delete all", opa_value_compare(builtin_json_remove(&obj5->hdr, &set_paths4->hdr), opa_object()) == 0);

    opa_object_t *obj6 = json_test_fixture_object4();

    opa_set_t *set_paths5 = opa_cast_set(opa_set());
    opa_set_add(set_paths5, opa_string_terminated("a/0/b"));
    opa_set_add(set_paths5, opa_string_terminated("a/1"));

    opa_object_t *o3 = opa_cast_object(opa_object());
    opa_object_insert(o3, opa_string_terminated("c"), opa_number_int(8));
    opa_array_t *a3 = opa_cast_array(opa_array());
    opa_array_append(a3, &o3->hdr);
    opa_object_t *expected4 = opa_cast_object(opa_object());
    opa_object_insert(expected4, opa_string_terminated("a"), &a3->hdr);

    test("jsonremove/arrays", opa_value_compare(builtin_json_remove(&obj6->hdr, &set_paths5->hdr), &expected4->hdr) == 0);

    opa_object_t *obj7 = json_test_fixture_object5();

    opa_set_t *set_paths6 = opa_cast_set(opa_set());
    opa_set_add(set_paths6, opa_string_terminated("a/0/1/2"));

    opa_object_t *expected5 = json_remove_get_exp_object1();

    test("jsonremove/object with number keys", opa_value_compare(builtin_json_remove(&obj7->hdr, &set_paths6->hdr), &expected5->hdr) == 0);

    opa_object_t *obj8 = json_test_fixture_object1();

    opa_array_t *a4 = opa_cast_array(opa_array());
    opa_array_append(a4, opa_string_terminated("a"));
    opa_array_append(a4, opa_string_terminated("b"));
    opa_array_append(a4, opa_string_terminated("c"));

    opa_array_t *a5 = opa_cast_array(opa_array());
    opa_array_append(a5, opa_string_terminated("e"));

    opa_set_t *set_paths7 = opa_cast_set(opa_set());
    opa_set_add(set_paths7, &a4->hdr);
    opa_set_add(set_paths7, &a5->hdr);

    test("jsonremove/arrays of roots", opa_value_compare(builtin_json_remove(&obj8->hdr, &set_paths7->hdr), &expected2->hdr) == 0);

    opa_object_t *obj9 = json_test_fixture_object6();

    opa_set_t *set_paths8 = opa_cast_set(opa_set());
    opa_set_add(set_paths8, opa_string_terminated("a/b/d"));
    opa_set_add(set_paths8, &a4->hdr);

    opa_object_t *expected6 = json_remove_get_exp_object2();

    test("jsonremove/mixed root types", opa_value_compare(builtin_json_remove(&obj9->hdr, &set_paths8->hdr), &expected6->hdr) == 0);

    test("jsonremove/error (invalid first operand - string)", opa_value_compare(builtin_json_remove(opa_string_terminated("a"),  opa_set()), NULL) == 0);

    test("jsonremove/error (invalid first operand - number)", opa_value_compare(builtin_json_remove(opa_number_int(22),  opa_set()), NULL) == 0);

    test("jsonremove/error (invalid first operand - boolean)", opa_value_compare(builtin_json_remove(opa_boolean(true),  opa_set()), NULL) == 0);

    test("jsonremove/error (invalid first operand - array)", opa_value_compare(builtin_json_remove(opa_array(),  opa_set()), NULL) == 0);

    test("jsonremove/error (invalid second operand - string)", opa_value_compare(builtin_json_remove(opa_object(), opa_string_terminated("a")), NULL) == 0);

    test("jsonremove/error (invalid second operand - number)", opa_value_compare(builtin_json_remove(opa_object(), opa_number_int(22)), NULL) == 0);

    test("jsonremove/error (invalid second operand - boolean)", opa_value_compare(builtin_json_remove(opa_object(), opa_boolean(true)), NULL) == 0);

    test("jsonremove/error (invalid second operand - object)", opa_value_compare(builtin_json_remove(opa_object(), opa_object()), NULL) == 0);

    opa_set_t *set_paths9 = opa_cast_set(opa_set());
    opa_set_add(set_paths9, opa_number_int(1));
    opa_set_add(set_paths9, opa_string_terminated("a"));

    test("jsonremove/error invalid paths type set with numbers", opa_value_compare(builtin_json_remove(opa_object(), &set_paths9->hdr), NULL) == 0);

    opa_set_t *set_paths10 = opa_cast_set(opa_set());
    opa_set_add(set_paths10, opa_string_terminated("a"));
    opa_set_add(set_paths10, &obj9->hdr);

    test("jsonremove/error invalid paths type set with objects", opa_value_compare(builtin_json_remove(opa_object(), &set_paths10->hdr), NULL) == 0);

    opa_array_t *array_paths2 = opa_cast_array(opa_array());
    opa_array_append(array_paths2, opa_string_terminated("a"));
    opa_array_append(array_paths2, opa_number_int(1));
    opa_array_append(array_paths2, opa_number_int(2));

    test("jsonremove/error invalid paths type array with numbers", opa_value_compare(builtin_json_remove(opa_object(), &array_paths2->hdr), NULL) == 0);

    opa_array_t *array_paths3 = opa_cast_array(opa_array());
    opa_array_append(array_paths3, opa_string_terminated("a"));
    opa_array_append(array_paths3, opa_object());

    test("jsonremove/error invalid paths type array with objects", opa_value_compare(builtin_json_remove(opa_object(), &array_paths3->hdr), NULL) == 0);

    opa_set_t *set_paths11 = opa_cast_set(opa_set());
    opa_set_add(set_paths11, opa_string_terminated("a/b"));
    opa_set_add(set_paths11, opa_string_terminated("e"));

    opa_object_t *expected7 = opa_cast_object(opa_object());
    opa_object_insert(expected7, opa_string_terminated("a"), opa_object());

    test("jsonremove/delete last in object", opa_value_compare(builtin_json_remove(&obj5->hdr, &set_paths11->hdr), &expected7->hdr) == 0);
}

WASM_EXPORT(test_json_filter)
void test_json_filter(void)
{
    opa_object_t *obj1 = json_test_fixture_object1();

    opa_set_t *set_paths1 = opa_cast_set(opa_set());
    opa_set_add(set_paths1, opa_string_terminated("a/b/c"));

    opa_object_t *o1 = opa_cast_object(opa_object());
    opa_object_insert(o1, opa_string_terminated("c"), opa_number_int(7));
    opa_object_t *o2 = opa_cast_object(opa_object());
    opa_object_insert(o2, opa_string_terminated("b"), &o1->hdr);

    opa_object_t *expected1 = opa_cast_object(opa_object());
    opa_object_insert(expected1, opa_string_terminated("a"), &o2->hdr);

    test("jsonfilter/base", opa_value_compare(builtin_json_filter(&obj1->hdr, &set_paths1->hdr), &expected1->hdr) == 0);

    opa_set_add(set_paths1, opa_string_terminated("e"));
    opa_object_insert(expected1, opa_string_terminated("e"), opa_number_int(9));

    test("jsonfilter/multiple roots", opa_value_compare(builtin_json_filter(&obj1->hdr, &set_paths1->hdr), &expected1->hdr) == 0);

    opa_array_t *array_paths1 = opa_cast_array(opa_array());
    opa_array_append(array_paths1, opa_string_terminated("a/b/c"));
    opa_array_append(array_paths1, opa_string_terminated("e"));

    test("jsonfilter/multiple roots array", opa_value_compare(builtin_json_filter(&obj1->hdr, &array_paths1->hdr), &expected1->hdr) == 0);

    opa_object_t *obj2 = json_test_fixture_object2();

    opa_set_t *set_paths2 = opa_cast_set(opa_set());
    opa_set_add(set_paths2, opa_string_terminated("a/b/c"));
    opa_set_add(set_paths2, opa_string_terminated("a/e"));

    opa_object_t *o3 = opa_cast_object(opa_object());
    opa_object_insert(o3, opa_string_terminated("b"), &o1->hdr);
    opa_object_insert(o3, opa_string_terminated("e"), opa_number_int(9));

    opa_object_t *expected2 = opa_cast_object(opa_object());
    opa_object_insert(expected2, opa_string_terminated("a"), &o3->hdr);

    test("jsonfilter/shared roots", opa_value_compare(builtin_json_filter(&obj2->hdr, &set_paths2->hdr), &expected2->hdr) == 0);

    opa_object_t *o4 = opa_cast_object(opa_object());
    opa_object_insert(o4, opa_string_terminated("b"), opa_number_int(7));

    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, opa_string_terminated("a"), &o4->hdr);

    opa_set_t *set_paths3 = opa_cast_set(opa_set());
    opa_set_add(set_paths3, opa_string_terminated("a"));
    opa_set_add(set_paths3, opa_string_terminated("a/b"));

    test("jsonfilter/conflict", opa_value_compare(builtin_json_filter(&obj3->hdr, &set_paths3->hdr), &obj3->hdr) == 0);

    test("jsonfilter/empty list", opa_value_compare(builtin_json_filter(&obj3->hdr, opa_set()), opa_object()) == 0);

    test("jsonfilter/empty object", opa_value_compare(builtin_json_filter(opa_object(), &set_paths3->hdr), opa_object()) == 0);

    opa_object_t *obj4 = json_test_fixture_object4();
    opa_set_t *set_paths4 = opa_cast_set(opa_set());
    opa_set_add(set_paths4, opa_string_terminated("a/0/b"));
    opa_set_add(set_paths4, opa_string_terminated("a/1"));

    opa_object_t *o5 = opa_cast_object(opa_object());
    opa_object_insert(o5, opa_string_terminated("b"), opa_number_int(7));

    opa_object_t *o6 = opa_cast_object(opa_object());
    opa_object_insert(o6, opa_string_terminated("d"), opa_number_int(9));

    opa_array_t *a1 = opa_cast_array(opa_array());
    opa_array_append(a1, &o5->hdr);
    opa_array_append(a1, &o6->hdr);

    opa_object_t *expected3 = opa_cast_object(opa_object());
    opa_object_insert(expected3, opa_string_terminated("a"), &a1->hdr);

    test("jsonfilter/arrays", opa_value_compare(builtin_json_filter(&obj4->hdr, &set_paths4->hdr), &expected3->hdr) == 0);

    opa_object_t *obj5 = json_test_fixture_object5();
    opa_set_t *set_paths5 = opa_cast_set(opa_set());
    opa_set_add(set_paths5, opa_string_terminated("a/0/1/2"));

    opa_array_t *a2 = opa_cast_array(opa_array());
    opa_array_append(a2, opa_string_terminated("d"));

    opa_object_t *o7 = opa_cast_object(opa_object());
    opa_object_insert(o7, opa_string_terminated("1"), &a2->hdr);

    opa_array_t *a3 = opa_cast_array(opa_array());
    opa_array_append(a3, &o7->hdr);

    opa_object_t *expected4 = opa_cast_object(opa_object());
    opa_object_insert(expected4, opa_string_terminated("a"), &a3->hdr);

    test("jsonfilter/object with number keys", opa_value_compare(builtin_json_filter(&obj5->hdr, &set_paths5->hdr), &expected4->hdr) == 0);

    opa_array_t *a4 = opa_cast_array(opa_array());
    opa_array_append(a4, opa_string_terminated("a"));
    opa_array_append(a4, opa_string_terminated("b"));
    opa_array_append(a4, opa_string_terminated("c"));

    opa_array_t *a5 = opa_cast_array(opa_array());
    opa_array_append(a5, opa_string_terminated("e"));

    opa_set_t *set_paths6 = opa_cast_set(opa_set());
    opa_set_add(set_paths6, &a4->hdr);
    opa_set_add(set_paths6, &a5->hdr);

    test("jsonfilter/arrays of roots", opa_value_compare(builtin_json_filter(&obj1->hdr, &set_paths6->hdr), &expected1->hdr) == 0);

    opa_object_t *obj6 = json_test_fixture_object6();

    opa_set_t *set_paths7 = opa_cast_set(opa_set());
    opa_set_add(set_paths7, opa_string_terminated("a/b/d"));
    opa_set_add(set_paths7, &a4->hdr);

    opa_object_t *o8 = opa_cast_object(opa_object());
    opa_object_insert(o8, opa_string_terminated("c"), opa_number_int(7));
    opa_object_insert(o8, opa_string_terminated("d"), opa_number_int(8));

    opa_object_t *o9 = opa_cast_object(opa_object());
    opa_object_insert(o9, opa_string_terminated("b"), &o8->hdr);

    opa_object_t *expected5 = opa_cast_object(opa_object());
    opa_object_insert(expected5, opa_string_terminated("a"), &o9->hdr);

    test("jsonfilter/mixed root types", opa_value_compare(builtin_json_filter(&obj6->hdr, &set_paths7->hdr), &expected5->hdr) == 0);

    test("jsonfilter/error (invalid first operand - string)", opa_value_compare(builtin_json_filter(opa_string_terminated("a"),  opa_set()), NULL) == 0);

    test("jsonfilter/error (invalid first operand - number)", opa_value_compare(builtin_json_filter(opa_number_int(22),  opa_set()), NULL) == 0);

    test("jsonfilter/error (invalid first operand - boolean)", opa_value_compare(builtin_json_filter(opa_boolean(true),  opa_set()), NULL) == 0);

    test("jsonfilter/error (invalid first operand - array)", opa_value_compare(builtin_json_filter(opa_array(),  opa_set()), NULL) == 0);

    test("jsonfilter/error (invalid second operand - string)", opa_value_compare(builtin_json_filter(opa_object(), opa_string_terminated("a")), NULL) == 0);

    test("jsonfilter/error (invalid second operand - number)", opa_value_compare(builtin_json_filter(opa_object(), opa_number_int(22)), NULL) == 0);

    test("jsonfilter/error (invalid second operand - boolean)", opa_value_compare(builtin_json_filter(opa_object(), opa_boolean(true)), NULL) == 0);

    test("jsonfilter/error (invalid second operand - object)", opa_value_compare(builtin_json_filter(opa_object(), opa_object()), NULL) == 0);
}

WASM_EXPORT(test_builtin_graph_reachable)
void test_builtin_graph_reachable(void)
{

    test("reachable/malformed graph", opa_value_compare(builtin_graph_reachable(opa_set(), opa_set()), NULL) == 0);

    opa_object_t *graph1 = opa_cast_object(opa_object());
    opa_set_t *initial1 = opa_cast_set(opa_set());
    opa_set_add(initial1, opa_string_terminated("a"));

    test("reachable/empty", opa_value_compare(builtin_graph_reachable(&graph1->hdr, &initial1->hdr), opa_set()) == 0);

    // graph -> {"a": {"b"}, "b": {"c"}, "c": {"a"}}
    opa_set_t *vertex1_1 = opa_cast_set(opa_set());
    opa_set_add(vertex1_1, opa_string_terminated("b"));
    opa_object_insert(graph1, opa_string_terminated("a"), &vertex1_1->hdr);

    opa_set_t *vertex2_1 = opa_cast_set(opa_set());
    opa_set_add(vertex2_1, opa_string_terminated("c"));
    opa_object_insert(graph1, opa_string_terminated("b"), &vertex2_1->hdr);

    opa_set_t *vertex3_1 = opa_cast_set(opa_set());
    opa_set_add(vertex3_1, opa_string_terminated("a"));
    opa_object_insert(graph1, opa_string_terminated("c"), &vertex3_1->hdr);

    opa_set_t *expected1 = opa_cast_set(opa_set());
    opa_set_add(expected1, opa_string_terminated("a"));
    opa_set_add(expected1, opa_string_terminated("b"));
    opa_set_add(expected1, opa_string_terminated("c"));

    test("reachable/cycle", opa_value_compare(builtin_graph_reachable(&graph1->hdr, &initial1->hdr), &expected1->hdr) == 0);

    // graph -> {"a": {"b", "c"}, "b": {"d"}, "c": {"d"}, "d": {}, "e": {"f"}, "f": {"e"}, "x": {"x"}}
    opa_object_t *graph2 = opa_cast_object(opa_object());
    opa_set_t *initial2 = opa_cast_set(opa_set());
    opa_set_add(initial2, opa_string_terminated("b"));
    opa_set_add(initial2, opa_string_terminated("e"));

    opa_set_t *vertex1_2 = opa_cast_set(opa_set());
    opa_set_add(vertex1_2, opa_string_terminated("b"));
    opa_set_add(vertex1_2, opa_string_terminated("c"));
    opa_object_insert(graph2, opa_string_terminated("a"), &vertex1_2->hdr);

    opa_set_t *vertex2_2 = opa_cast_set(opa_set());
    opa_set_add(vertex2_2, opa_string_terminated("d"));
    opa_object_insert(graph2, opa_string_terminated("b"), &vertex2_2->hdr);
    opa_object_insert(graph2, opa_string_terminated("c"), &vertex2_2->hdr);

    opa_object_insert(graph2, opa_string_terminated("d"), opa_set());

    opa_set_t *vertex3_2 = opa_cast_set(opa_set());
    opa_set_add(vertex3_2, opa_string_terminated("f"));
    opa_object_insert(graph2, opa_string_terminated("e"), &vertex3_2->hdr);

    opa_set_t *vertex4_2 = opa_cast_set(opa_set());
    opa_set_add(vertex4_2, opa_string_terminated("e"));
    opa_object_insert(graph2, opa_string_terminated("f"), &vertex4_2->hdr);

    opa_set_t *vertex5_2 = opa_cast_set(opa_set());
    opa_set_add(vertex5_2, opa_string_terminated("x"));
    opa_object_insert(graph2, opa_string_terminated("x"), &vertex5_2->hdr);

    opa_set_t *expected2 = opa_cast_set(opa_set());
    opa_set_add(expected2, opa_string_terminated("b"));
    opa_set_add(expected2, opa_string_terminated("d"));
    opa_set_add(expected2, opa_string_terminated("e"));
    opa_set_add(expected2, opa_string_terminated("f"));

    test("reachable/components", opa_value_compare(builtin_graph_reachable(&graph2->hdr, &initial2->hdr), &expected2->hdr) == 0);

    // graph -> {"a": ["b"], "b": ["c"], "c": ["a"]}
    opa_object_t *graph3 = opa_cast_object(opa_object());
    opa_array_t *initial3 = opa_cast_array(opa_array());
    opa_array_append(initial3, opa_string_terminated("a"));

    opa_array_t *vertex1_3 = opa_cast_array(opa_array());
    opa_array_append(vertex1_3, opa_string_terminated("b"));
    opa_object_insert(graph3, opa_string_terminated("a"), &vertex1_3->hdr);

    opa_array_t *vertex2_3 = opa_cast_array(opa_array());
    opa_array_append(vertex2_3, opa_string_terminated("c"));
    opa_object_insert(graph3, opa_string_terminated("b"), &vertex2_3->hdr);

    opa_array_t *vertex3_3 = opa_cast_array(opa_array());
    opa_array_append(vertex3_3, opa_string_terminated("a"));
    opa_object_insert(graph3, opa_string_terminated("c"), &vertex3_3->hdr);

    opa_set_t *expected3 = opa_cast_set(opa_set());
    opa_set_add(expected3, opa_string_terminated("a"));
    opa_set_add(expected3, opa_string_terminated("b"));
    opa_set_add(expected3, opa_string_terminated("c"));

    test("reachable/arrays", opa_value_compare(builtin_graph_reachable(&graph3->hdr, &initial3->hdr), &expected3->hdr) == 0);

    test("reachable/malformed initial nodes", opa_value_compare(builtin_graph_reachable(&graph3->hdr, opa_string_terminated("foo")), NULL) == 0);

    // graph -> {"a": null}
    opa_object_t *graph4 = opa_cast_object(opa_object());
    opa_object_insert(graph4, opa_string_terminated("a"), opa_null());

    opa_set_t *expected4 = opa_cast_set(opa_set());
    opa_set_add(expected4, opa_string_terminated("a"));

    test("reachable/null edge", opa_value_compare(builtin_graph_reachable(&graph4->hdr, &initial3->hdr), &expected4->hdr) == 0);
}

WASM_EXPORT(test_strings)
void test_strings(void)
{
    opa_array_t *any_prefix_match_string_arr_1 = opa_cast_array(opa_array());
    opa_array_append(any_prefix_match_string_arr_1, opa_string_terminated("a/b/c"));
    opa_array_append(any_prefix_match_string_arr_1, opa_string_terminated("e/f/g"));

    opa_array_t *any_prefix_match_prefixes_arr_11 = opa_cast_array(opa_array());
    opa_array_append(any_prefix_match_prefixes_arr_11, opa_string_terminated("g/b"));
    opa_array_append(any_prefix_match_prefixes_arr_11, opa_string_terminated("a/"));

    opa_array_t *any_prefix_match_prefixes_arr_12 = opa_cast_array(opa_array());
    opa_array_append(any_prefix_match_prefixes_arr_12, opa_string_terminated("g/b"));
    opa_array_append(any_prefix_match_prefixes_arr_12, opa_string_terminated("b/"));

    opa_array_t *any_prefix_match_string_arr_2 = opa_cast_array(opa_array());
    opa_array_t *any_prefix_match_prefixes_arr_2 = opa_cast_array(opa_array());

    opa_set_t *any_prefix_match_string_set_1 = opa_cast_set(opa_set());
    opa_set_add(any_prefix_match_string_set_1, opa_string_terminated("a/b/c"));
    opa_set_add(any_prefix_match_string_set_1, opa_string_terminated("e/f/g"));

    opa_set_t *any_prefix_match_prefixes_set_11 = opa_cast_set(opa_set());
    opa_set_add(any_prefix_match_prefixes_set_11, opa_string_terminated("g/b"));
    opa_set_add(any_prefix_match_prefixes_set_11, opa_string_terminated("a/"));

    opa_set_t *any_prefix_match_prefixes_set_12 = opa_cast_set(opa_set());
    opa_set_add(any_prefix_match_prefixes_set_12, opa_string_terminated("g/b"));
    opa_set_add(any_prefix_match_prefixes_set_12, opa_string_terminated("b/"));

    opa_set_t *any_prefix_match_string_set_2 = opa_cast_set(opa_set());
    opa_set_t *any_prefix_match_prefixes_set_2 = opa_cast_set(opa_set());

    test("any_prefix_match/__", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated(""), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("any_prefix_match/_a", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated(""), opa_string_terminated("a")), opa_boolean(false)) == 0);
    test("any_prefix_match/a_", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("a"), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("any_prefix_match/aa", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("a"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("any_prefix_match/ab", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("a"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("any_prefix_match/aab", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("a"), opa_string_terminated("ab")), opa_boolean(false)) == 0);
    test("any_prefix_match/aba", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("ab"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("any_prefix_match/aab", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("aa"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("any_prefix_match/abab", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("any_prefix_match/abaa", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_boolean(false)) == 0);
    test("any_prefix_match/abcab", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("abc"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("any_prefix_match/abcac", opa_value_compare(opa_strings_any_prefix_match(opa_string_terminated("abc"), opa_string_terminated("ac")), opa_boolean(false)) == 0);
    test("any_prefix_match/arr11", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_arr_1->hdr, &any_prefix_match_prefixes_arr_11->hdr), opa_boolean(true)) == 0);
    test("any_prefix_match/arr12", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_arr_1->hdr, &any_prefix_match_prefixes_arr_12->hdr), opa_boolean(false)) == 0);
    test("any_prefix_match/arr2", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_arr_2->hdr, &any_prefix_match_prefixes_arr_2->hdr), opa_boolean(false)) == 0);
    test("any_prefix_match/set11", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_set_1->hdr, &any_prefix_match_prefixes_set_11->hdr), opa_boolean(true)) == 0);
    test("any_prefix_match/set12", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_set_1->hdr, &any_prefix_match_prefixes_set_12->hdr), opa_boolean(false)) == 0);
    test("any_prefix_match/set2", opa_value_compare(opa_strings_any_prefix_match(&any_prefix_match_string_set_2->hdr, &any_prefix_match_prefixes_set_2->hdr), opa_boolean(false)) == 0);

    opa_array_t *any_suffix_match_string_arr_1 = opa_cast_array(opa_array());
    opa_array_append(any_suffix_match_string_arr_1, opa_string_terminated("a/b/c"));
    opa_array_append(any_suffix_match_string_arr_1, opa_string_terminated("e/f/g"));

    opa_array_t *any_suffix_match_suffixes_arr_11 = opa_cast_array(opa_array());
    opa_array_append(any_suffix_match_suffixes_arr_11, opa_string_terminated("g/b"));
    opa_array_append(any_suffix_match_suffixes_arr_11, opa_string_terminated("/c"));

    opa_array_t *any_suffix_match_suffixes_arr_12 = opa_cast_array(opa_array());
    opa_array_append(any_suffix_match_suffixes_arr_12, opa_string_terminated("g/b"));
    opa_array_append(any_suffix_match_suffixes_arr_12, opa_string_terminated("/b"));

    opa_array_t *any_suffix_match_string_arr_2 = opa_cast_array(opa_array());
    opa_array_t *any_suffix_match_suffixes_arr_2 = opa_cast_array(opa_array());

    opa_set_t *any_suffix_match_string_set_1 = opa_cast_set(opa_set());
    opa_set_add(any_suffix_match_string_set_1, opa_string_terminated("a/b/c"));
    opa_set_add(any_suffix_match_string_set_1, opa_string_terminated("e/f/g"));

    opa_set_t *any_suffix_match_suffixes_set_11 = opa_cast_set(opa_set());
    opa_set_add(any_suffix_match_suffixes_set_11, opa_string_terminated("g/b"));
    opa_set_add(any_suffix_match_suffixes_set_11, opa_string_terminated("/c"));

    opa_set_t *any_suffix_match_suffixes_set_12 = opa_cast_set(opa_set());
    opa_set_add(any_suffix_match_suffixes_set_12, opa_string_terminated("g/b"));
    opa_set_add(any_suffix_match_suffixes_set_12, opa_string_terminated("/b"));

    opa_set_t *any_suffix_match_string_set_2 = opa_cast_set(opa_set());
    opa_set_t *any_suffix_match_suffixes_set_2 = opa_cast_set(opa_set());

    test("any_suffix_match/__", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated(""), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("any_suffix_match/_a", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated(""), opa_string_terminated("a")), opa_boolean(false)) == 0);
    test("any_suffix_match/a_", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("a"), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("any_suffix_match/aa", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("a"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("any_suffix_match/ab", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("a"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("any_suffix_match/aab", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("a"), opa_string_terminated("ab")), opa_boolean(false)) == 0);
    test("any_suffix_match/abb", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("ab"), opa_string_terminated("b")), opa_boolean(true)) == 0);
    test("any_suffix_match/aab", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("aa"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("any_suffix_match/abab", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("any_suffix_match/abaa", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_boolean(false)) == 0);
    test("any_suffix_match/abcbc", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("abc"), opa_string_terminated("bc")), opa_boolean(true)) == 0);
    test("any_suffix_match/abcbd", opa_value_compare(opa_strings_any_suffix_match(opa_string_terminated("abc"), opa_string_terminated("bd")), opa_boolean(false)) == 0);
    test("any_suffix_match/arr11", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_arr_1->hdr, &any_suffix_match_suffixes_arr_11->hdr), opa_boolean(true)) == 0);
    test("any_suffix_match/arr12", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_arr_1->hdr, &any_suffix_match_suffixes_arr_12->hdr), opa_boolean(false)) == 0);
    test("any_suffix_match/arr2", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_arr_2->hdr, &any_suffix_match_suffixes_arr_2->hdr), opa_boolean(false)) == 0);
    test("any_suffix_match/set11", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_set_1->hdr, &any_suffix_match_suffixes_set_11->hdr), opa_boolean(true)) == 0);
    test("any_suffix_match/set12", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_set_1->hdr, &any_suffix_match_suffixes_set_12->hdr), opa_boolean(false)) == 0);
    test("any_suffix_match/set2", opa_value_compare(opa_strings_any_suffix_match(&any_suffix_match_string_set_2->hdr, &any_suffix_match_suffixes_set_2->hdr), opa_boolean(false)) == 0);

    opa_value *join = opa_string_terminated("--");

    opa_array_t *arr0 = opa_cast_array(opa_array());

    opa_array_t *arr1 = opa_cast_array(opa_array());
    opa_array_append(arr1, opa_string_terminated("foo"));

    opa_array_t *arr2 = opa_cast_array(opa_array());
    opa_array_append(arr2, opa_string_terminated("foo"));
    opa_array_append(arr2, opa_string_terminated("bar"));

    opa_set_t *set0 = opa_cast_set(opa_set());

    opa_set_t *set1 = opa_cast_set(opa_set());
    opa_set_add(set1, opa_string_terminated("foo"));

    opa_set_t *set2 = opa_cast_set(opa_set());
    opa_set_add(set2, opa_string_terminated("foo"));
    opa_set_add(set2, opa_string_terminated("bar"));

    test("concat/array0", opa_value_compare(opa_strings_concat(join, &arr0->hdr), opa_string_terminated("")) == 0);
    test("concat/array1", opa_value_compare(opa_strings_concat(join, &arr1->hdr), opa_string_terminated("foo")) == 0);
    test("concat/array2", opa_value_compare(opa_strings_concat(join, &arr2->hdr), opa_string_terminated("foo--bar")) == 0);
    test("concat/set0", opa_value_compare(opa_strings_concat(join, &set0->hdr), opa_string_terminated("")) == 0);
    test("concat/set1", opa_value_compare(opa_strings_concat(join, &set1->hdr), opa_string_terminated("foo")) == 0);
    test("concat/set2", opa_value_compare(opa_strings_concat(join, &set2->hdr), opa_string_terminated("bar--foo")) == 0);

    test("contains/__", opa_value_compare(opa_strings_contains(opa_string_terminated(""), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("contains/_a", opa_value_compare(opa_strings_contains(opa_string_terminated(""), opa_string_terminated("a")), opa_boolean(false)) == 0);
    test("contains/a_", opa_value_compare(opa_strings_contains(opa_string_terminated("a"), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("contains/aa", opa_value_compare(opa_strings_contains(opa_string_terminated("a"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("contains/ab", opa_value_compare(opa_strings_contains(opa_string_terminated("a"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("contains/aab", opa_value_compare(opa_strings_contains(opa_string_terminated("a"), opa_string_terminated("ab")), opa_boolean(false)) == 0);
    test("contains/abb", opa_value_compare(opa_strings_contains(opa_string_terminated("ab"), opa_string_terminated("b")), opa_boolean(true)) == 0);
    test("contains/aab", opa_value_compare(opa_strings_contains(opa_string_terminated("aa"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("contains/abab", opa_value_compare(opa_strings_contains(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("contains/abaa", opa_value_compare(opa_strings_contains(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_boolean(false)) == 0);
    test("contains/abcbc", opa_value_compare(opa_strings_contains(opa_string_terminated("abc"), opa_string_terminated("bc")), opa_boolean(true)) == 0);
    test("contains/abcbd", opa_value_compare(opa_strings_contains(opa_string_terminated("abc"), opa_string_terminated("bd")), opa_boolean(false)) == 0);

    test("endswith/__", opa_value_compare(opa_strings_endswith(opa_string_terminated(""), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("endswith/_a", opa_value_compare(opa_strings_endswith(opa_string_terminated(""), opa_string_terminated("a")), opa_boolean(false)) == 0);
    test("endswith/a_", opa_value_compare(opa_strings_endswith(opa_string_terminated("a"), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("endswith/aa", opa_value_compare(opa_strings_endswith(opa_string_terminated("a"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("endswith/ab", opa_value_compare(opa_strings_endswith(opa_string_terminated("a"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("endswith/aab", opa_value_compare(opa_strings_endswith(opa_string_terminated("a"), opa_string_terminated("ab")), opa_boolean(false)) == 0);
    test("endswith/abb", opa_value_compare(opa_strings_endswith(opa_string_terminated("ab"), opa_string_terminated("b")), opa_boolean(true)) == 0);
    test("endswith/aab", opa_value_compare(opa_strings_endswith(opa_string_terminated("aa"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("endswith/abab", opa_value_compare(opa_strings_endswith(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("endswith/abaa", opa_value_compare(opa_strings_endswith(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_boolean(false)) == 0);
    test("endswith/abcbc", opa_value_compare(opa_strings_endswith(opa_string_terminated("abc"), opa_string_terminated("bc")), opa_boolean(true)) == 0);
    test("endswith/abcbd", opa_value_compare(opa_strings_endswith(opa_string_terminated("abc"), opa_string_terminated("bd")), opa_boolean(false)) == 0);

    test("format_int/2_0", opa_value_compare(opa_strings_format_int(opa_number_float(0), opa_number_int(2)), opa_string_terminated("0")) == 0);
    test("format_int/2_1", opa_value_compare(opa_strings_format_int(opa_number_float(1), opa_number_int(2)), opa_string_terminated("1")) == 0);
    test("format_int/2_-1", opa_value_compare(opa_strings_format_int(opa_number_float(-1), opa_number_int(2)), opa_string_terminated("-1")) == 0);
    test("format_int/2_2", opa_value_compare(opa_strings_format_int(opa_number_float(2), opa_number_int(2)), opa_string_terminated("10")) == 0);
    test("format_int/2_7", opa_value_compare(opa_strings_format_int(opa_number_float(7), opa_number_int(2)), opa_string_terminated("111")) == 0);
    test("format_int/8_0", opa_value_compare(opa_strings_format_int(opa_number_float(0), opa_number_int(8)), opa_string_terminated("0")) == 0);
    test("format_int/8_1", opa_value_compare(opa_strings_format_int(opa_number_float(1), opa_number_int(8)), opa_string_terminated("1")) == 0);
    test("format_int/8_-1", opa_value_compare(opa_strings_format_int(opa_number_float(-1), opa_number_int(8)), opa_string_terminated("-1")) == 0);
    test("format_int/8_8", opa_value_compare(opa_strings_format_int(opa_number_float(8), opa_number_int(8)), opa_string_terminated("10")) == 0);
    test("format_int/8_9", opa_value_compare(opa_strings_format_int(opa_number_float(9), opa_number_int(8)), opa_string_terminated("11")) == 0);
    test("format_int/10_0", opa_value_compare(opa_strings_format_int(opa_number_float(0), opa_number_int(10)), opa_string_terminated("0")) == 0);
    test("format_int/10_1", opa_value_compare(opa_strings_format_int(opa_number_float(1), opa_number_int(10)), opa_string_terminated("1")) == 0);
    test("format_int/10_-1", opa_value_compare(opa_strings_format_int(opa_number_float(-1), opa_number_int(10)), opa_string_terminated("-1")) == 0);
    test("format_int/10_10", opa_value_compare(opa_strings_format_int(opa_number_float(10), opa_number_int(10)), opa_string_terminated("10")) == 0);
    test("format_int/10_11", opa_value_compare(opa_strings_format_int(opa_number_float(11), opa_number_int(10)), opa_string_terminated("11")) == 0);
    test("format_int/16_0", opa_value_compare(opa_strings_format_int(opa_number_float(0), opa_number_int(16)), opa_string_terminated("0")) == 0);
    test("format_int/16_1", opa_value_compare(opa_strings_format_int(opa_number_float(1), opa_number_int(16)), opa_string_terminated("1")) == 0);
    test("format_int/16_-1", opa_value_compare(opa_strings_format_int(opa_number_float(-1), opa_number_int(16)), opa_string_terminated("-1")) == 0);
    test("format_int/16_15.5", opa_value_compare(opa_strings_format_int(opa_number_float(15.5), opa_number_int(16)), opa_string_terminated("f")) == 0);
    test("format_int/16_-15.5", opa_value_compare(opa_strings_format_int(opa_number_float(-15.5), opa_number_int(16)), opa_string_terminated("-f")) == 0);
    test("format_int/16_16", opa_value_compare(opa_strings_format_int(opa_number_float(16), opa_number_int(16)), opa_string_terminated("10")) == 0);
    test("format_int/16_31", opa_value_compare(opa_strings_format_int(opa_number_float(31), opa_number_int(16)), opa_string_terminated("1f")) == 0);

    test("indexof/__", opa_value_compare(opa_strings_indexof(opa_string_terminated(""), opa_string_terminated("")), opa_number_int(0)) == 0);
    test("indexof/_a", opa_value_compare(opa_strings_indexof(opa_string_terminated(""), opa_string_terminated("a")), opa_number_int(-1)) == 0);
    test("indexof/a_", opa_value_compare(opa_strings_indexof(opa_string_terminated("a"), opa_string_terminated("")), opa_number_int(0)) == 0);
    test("indexof/aa", opa_value_compare(opa_strings_indexof(opa_string_terminated("a"), opa_string_terminated("a")), opa_number_int(0)) == 0);
    test("indexof/ab", opa_value_compare(opa_strings_indexof(opa_string_terminated("a"), opa_string_terminated("b")), opa_number_int(-1)) == 0);
    test("indexof/aab", opa_value_compare(opa_strings_indexof(opa_string_terminated("a"), opa_string_terminated("ab")), opa_number_int(-1)) == 0);
    test("indexof/abb", opa_value_compare(opa_strings_indexof(opa_string_terminated("ab"), opa_string_terminated("b")), opa_number_int(1)) == 0);
    test("indexof/aab", opa_value_compare(opa_strings_indexof(opa_string_terminated("aa"), opa_string_terminated("b")), opa_number_int(-1)) == 0);
    test("indexof/abab", opa_value_compare(opa_strings_indexof(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_number_int(0)) == 0);
    test("indexof/abaa", opa_value_compare(opa_strings_indexof(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_number_int(-1)) == 0);
    test("indexof/abcbc", opa_value_compare(opa_strings_indexof(opa_string_terminated("abc"), opa_string_terminated("bc")), opa_number_int(1)) == 0);
    test("indexof/abcbd", opa_value_compare(opa_strings_indexof(opa_string_terminated("abc"), opa_string_terminated("bd")), opa_number_int(-1)) == 0);
    test("indexof/unicode", opa_value_compare(opa_strings_indexof(opa_string_terminated("\xC3\xA5\xC3\xA4\xC3\xB6"), opa_string_terminated("\xC3\xB6")), opa_number_int(2)) == 0);

    test("replace/___", opa_value_compare(opa_strings_replace(opa_string_terminated(""), opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("replace/_ab", opa_value_compare(opa_strings_replace(opa_string_terminated(""), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("")) == 0);
    test("replace/aab", opa_value_compare(opa_strings_replace(opa_string_terminated("a"), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("b")) == 0);
    test("replace/cab", opa_value_compare(opa_strings_replace(opa_string_terminated("c"), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("c")) == 0);
    test("replace/aaab", opa_value_compare(opa_strings_replace(opa_string_terminated("aa"), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("bb")) == 0);
    test("replace/acaab", opa_value_compare(opa_strings_replace(opa_string_terminated("aca"), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("bcb")) == 0);
    test("replace/acaabd", opa_value_compare(opa_strings_replace(opa_string_terminated("aca"), opa_string_terminated("a"), opa_string_terminated("bd")), opa_string_terminated("bdcbd")) == 0);
    test("replace/cacab", opa_value_compare(opa_strings_replace(opa_string_terminated("cac"), opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("cbc")) == 0);
    test("replace/cacabd", opa_value_compare(opa_strings_replace(opa_string_terminated("cac"), opa_string_terminated("a"), opa_string_terminated("bd")), opa_string_terminated("cbdc")) == 0);

    test("reverse/abc", opa_value_compare(opa_strings_reverse(opa_string_terminated("abc")), opa_string_terminated("cba")) == 0);
    test("reverse/unicode", opa_value_compare(opa_strings_reverse(opa_string_terminated("1😀𝛾")), opa_string_terminated("𝛾😀1"))== 0);
    test("reverse/___", opa_value_compare(opa_strings_reverse(opa_string_terminated("")), opa_string_terminated("")) == 0);

    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("a"), opa_string_terminated("b"));
    opa_object_insert(obj2, opa_string_terminated("c"), opa_string_terminated("d"));

    test("replace_n/empty", opa_value_compare(opa_strings_replace_n(opa_object(), opa_string_terminated("a")), opa_string_terminated("a")) == 0);
    test("replace_n/two", opa_value_compare(opa_strings_replace_n(&obj2->hdr, opa_string_terminated("ac")), opa_string_terminated("bd")) == 0);

    opa_array_t *arr2b = opa_cast_array(opa_array());
    opa_array_append(arr2b, opa_string_terminated(""));
    opa_array_append(arr2b, opa_string_terminated("foo"));

    opa_array_t *arr2c = opa_cast_array(opa_array());
    opa_array_append(arr2c, opa_string_terminated("foo"));
    opa_array_append(arr2c, opa_string_terminated(""));

    opa_array_t *arr3 = opa_cast_array(opa_array());
    opa_array_append(arr3, opa_string_terminated("foo"));
    opa_array_append(arr3, opa_string_terminated("bar"));
    opa_array_append(arr3, opa_string_terminated("baz"));

    test("split/one", opa_value_compare(opa_strings_split(opa_string_terminated("foo"), opa_string_terminated(",")), &arr1->hdr) == 0);
    test("split/two_a", opa_value_compare(opa_strings_split(opa_string_terminated("foo,bar"), opa_string_terminated(",")), &arr2->hdr) == 0);
    test("split/two_b", opa_value_compare(opa_strings_split(opa_string_terminated(",,foo"), opa_string_terminated(",,")), &arr2b->hdr) == 0);
    test("split/two_c", opa_value_compare(opa_strings_split(opa_string_terminated("foo,,"), opa_string_terminated(",,")), &arr2c->hdr) == 0);
    test("split/three", opa_value_compare(opa_strings_split(opa_string_terminated("foo,,bar,,baz"), opa_string_terminated(",,")), &arr3->hdr) == 0);

    opa_array_t *arr4 = opa_cast_array(opa_array());
    opa_array_append(arr4, opa_string_terminated("f"));
    opa_array_append(arr4, opa_string_terminated("o"));
    opa_array_append(arr4, opa_string_terminated("o"));

    opa_array_t *arr5 = opa_cast_array(opa_array());
    opa_array_append(arr5, opa_string_terminated("f"));
    opa_array_append(arr5, opa_string_terminated("\xE2\x82\xAC")); // euro symbol
    opa_array_append(arr5, opa_string_terminated("o"));

    test("split/ascii", opa_value_compare(opa_strings_split(opa_string_terminated("foo"), opa_string_terminated("")), &arr4->hdr) == 0);
    test("split/utf8", opa_value_compare(opa_strings_split(opa_string_terminated("f\xE2\x82\xACo"), opa_string_terminated("")), &arr5->hdr) == 0);

    opa_array_t *arr6 = opa_cast_array(opa_array());
    opa_array_append(arr6, opa_string_terminated(""));
    test("split/empty", opa_value_compare(opa_strings_split(opa_string_terminated(""), opa_string_terminated(",")), &arr6->hdr) == 0);

    test("startswith/__", opa_value_compare(opa_strings_startswith(opa_string_terminated(""), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("startswith/_a", opa_value_compare(opa_strings_startswith(opa_string_terminated(""), opa_string_terminated("a")), opa_boolean(false)) == 0);
    test("startswith/a_", opa_value_compare(opa_strings_startswith(opa_string_terminated("a"), opa_string_terminated("")), opa_boolean(true)) == 0);
    test("startswith/aa", opa_value_compare(opa_strings_startswith(opa_string_terminated("a"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("startswith/ab", opa_value_compare(opa_strings_startswith(opa_string_terminated("a"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("startswith/aab", opa_value_compare(opa_strings_startswith(opa_string_terminated("a"), opa_string_terminated("ab")), opa_boolean(false)) == 0);
    test("startswith/aba", opa_value_compare(opa_strings_startswith(opa_string_terminated("ab"), opa_string_terminated("a")), opa_boolean(true)) == 0);
    test("startswith/aab", opa_value_compare(opa_strings_startswith(opa_string_terminated("aa"), opa_string_terminated("b")), opa_boolean(false)) == 0);
    test("startswith/abab", opa_value_compare(opa_strings_startswith(opa_string_terminated("ab"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("startswith/abaa", opa_value_compare(opa_strings_startswith(opa_string_terminated("ab"), opa_string_terminated("aa")), opa_boolean(false)) == 0);
    test("startswith/abcab", opa_value_compare(opa_strings_startswith(opa_string_terminated("abc"), opa_string_terminated("ab")), opa_boolean(true)) == 0);
    test("startswith/abcac", opa_value_compare(opa_strings_startswith(opa_string_terminated("abc"), opa_string_terminated("ac")), opa_boolean(false)) == 0);

    test("substring/_00", opa_value_compare(opa_strings_substring(opa_string_terminated(""), opa_number_int(0), opa_number_int(0)), opa_string_terminated("")) == 0);
    test("substring/_0-1", opa_value_compare(opa_strings_substring(opa_string_terminated(""), opa_number_int(0), opa_number_int(-1)), opa_string_terminated("")) == 0);
    test("substring/_10", opa_value_compare(opa_strings_substring(opa_string_terminated(""), opa_number_int(1), opa_number_int(0)), opa_string_terminated("")) == 0);
    test("substring/_1-1", opa_value_compare(opa_strings_substring(opa_string_terminated(""), opa_number_int(1), opa_number_int(-1)), opa_string_terminated("")) == 0);
    test("substring/abc1-1", opa_value_compare(opa_strings_substring(opa_string_terminated("abc"), opa_number_int(1), opa_number_int(-1)), opa_string_terminated("bc")) == 0);
    test("substring/abc10", opa_value_compare(opa_strings_substring(opa_string_terminated("abc"), opa_number_int(1), opa_number_int(0)), opa_string_terminated("")) == 0);
    test("substring/abc11", opa_value_compare(opa_strings_substring(opa_string_terminated("abc"), opa_number_int(1), opa_number_int(1)), opa_string_terminated("b")) == 0);
    test("substring/abc12", opa_value_compare(opa_strings_substring(opa_string_terminated("abc"), opa_number_int(1), opa_number_int(2)), opa_string_terminated("bc")) == 0);
    test("substring/abc41", opa_value_compare(opa_strings_substring(opa_string_terminated("abc"), opa_number_int(4), opa_number_int(1)), opa_string_terminated("")) == 0);
    test("substring/unicode", opa_value_compare(opa_strings_substring(opa_string_terminated("\xC3\xA5\xC3\xA4\xC3\xB6\x7A"), opa_number_int(1), opa_number_int(1)), opa_string_terminated("\xC3\xA4")) == 0);
    test("substring/unicode", opa_value_compare(opa_strings_substring(opa_string_terminated("\xC3\xA5\xC3\xA4\xC3\xB6\x7A"), opa_number_int(1), opa_number_int(2)), opa_string_terminated("\xC3\xA4\xC3\xB6")) == 0);
    test("substring/unicode", opa_value_compare(opa_strings_substring(opa_string_terminated("\xC3\xA5\xC3\xA4\xC3\xB6\x7A"), opa_number_int(2), opa_number_int(-1)), opa_string_terminated("\xC3\xB6\x7A")) == 0);

    test("trim/__", opa_value_compare(opa_strings_trim(opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim/abcba", opa_value_compare(opa_strings_trim(opa_string_terminated("abc"), opa_string_terminated("ba")), opa_string_terminated("c")) == 0);

    test("trim_left/__", opa_value_compare(opa_strings_trim_left(opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim_left/_a", opa_value_compare(opa_strings_trim_left(opa_string_terminated(""), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_left/a_", opa_value_compare(opa_strings_trim_left(opa_string_terminated("a"), opa_string_terminated("")), opa_string_terminated("a")) == 0);
    test("trim_left/aa", opa_value_compare(opa_strings_trim_left(opa_string_terminated("a"), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_left/ab", opa_value_compare(opa_strings_trim_left(opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);
    test("trim_left/aba", opa_value_compare(opa_strings_trim_left(opa_string_terminated("ab"), opa_string_terminated("a")), opa_string_terminated("b")) == 0);
    test("trim_left/abcba", opa_value_compare(opa_strings_trim_left(opa_string_terminated("abc"), opa_string_terminated("ba")), opa_string_terminated("c")) == 0);
    test("trim_left/aeuro dceuro ", opa_value_compare(opa_strings_trim_left(opa_string_terminated("a\xE2\x82\xAC d"), opa_string_terminated("ca\xE2\x82\xAC ")), opa_string_terminated("d")) == 0);

    test("trim_prefix/__", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim_prefix/_a", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated(""), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_prefix/a_", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated("a"), opa_string_terminated("")), opa_string_terminated("a")) == 0);
    test("trim_prefix/aa", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated("a"), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_prefix/ab", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);
    test("trim_prefix/aba", opa_value_compare(opa_strings_trim_prefix(opa_string_terminated("ab"), opa_string_terminated("a")), opa_string_terminated("b")) == 0);

    test("trim_right/__", opa_value_compare(opa_strings_trim_right(opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim_right/_a", opa_value_compare(opa_strings_trim_right(opa_string_terminated(""), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_right/a_", opa_value_compare(opa_strings_trim_right(opa_string_terminated("a"), opa_string_terminated("")), opa_string_terminated("a")) == 0);
    test("trim_right/aa", opa_value_compare(opa_strings_trim_right(opa_string_terminated("a"), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_right/ab", opa_value_compare(opa_strings_trim_right(opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);
    test("trim_right/abb", opa_value_compare(opa_strings_trim_right(opa_string_terminated("ab"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);
    test("trim_right/abccb", opa_value_compare(opa_strings_trim_right(opa_string_terminated("abc"), opa_string_terminated("cb")), opa_string_terminated("a")) == 0);
    test("trim_right/daeuro ceuro ", opa_value_compare(opa_strings_trim_right(opa_string_terminated("da\xE2\x82\xAC "), opa_string_terminated("ca\xE2\x82\xAC ")), opa_string_terminated("d")) == 0);

    test("trim_suffix/__", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated(""), opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim_suffix/_a", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated(""), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_suffix/a_", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated("a"), opa_string_terminated("")), opa_string_terminated("a")) == 0);
    test("trim_suffix/aa", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated("a"), opa_string_terminated("a")), opa_string_terminated("")) == 0);
    test("trim_suffix/ab", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated("a"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);
    test("trim_suffix/abb", opa_value_compare(opa_strings_trim_suffix(opa_string_terminated("ab"), opa_string_terminated("b")), opa_string_terminated("a")) == 0);

    test("trim_space/_", opa_value_compare(opa_strings_trim_space(opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("trim_space/a", opa_value_compare(opa_strings_trim_space(opa_string_terminated("a")), opa_string_terminated("a")) == 0);
    test("trim_space/_a", opa_value_compare(opa_strings_trim_space(opa_string_terminated(" a")), opa_string_terminated("a")) == 0);
    test("trim_space/a_", opa_value_compare(opa_strings_trim_space(opa_string_terminated("a ")), opa_string_terminated("a")) == 0);
    test("trim_space/_a_", opa_value_compare(opa_strings_trim_space(opa_string_terminated(" a ")), opa_string_terminated("a")) == 0);
    test("trim_space/______a_b_c______", opa_value_compare(opa_strings_trim_space(opa_string_terminated("\t\n\v\f\r a b c \r\f\v\n\t")), opa_string_terminated("a b c")) == 0);
    test("trim_space/euro", opa_value_compare(opa_strings_trim_space(opa_string_terminated("\xE2\x82\xAC")), opa_string_terminated("\xE2\x82\xAC")) == 0);
    test("trim_space/_euro_", opa_value_compare(opa_strings_trim_space(opa_string_terminated(" \xE2\x82\xAC ")), opa_string_terminated("\xE2\x82\xAC")) == 0);
    test("trim_space/a_euro_", opa_value_compare(opa_strings_trim_space(opa_string_terminated("a \xE2\x82\xAC ")), opa_string_terminated("a \xE2\x82\xAC")) == 0);
    test("trim_space/_euro_a", opa_value_compare(opa_strings_trim_space(opa_string_terminated(" \xE2\x82\xAC a")), opa_string_terminated("\xE2\x82\xAC a")) == 0);
    test("trim_space/ogham_a", opa_value_compare(opa_strings_trim_space(opa_string_terminated("\xe1\x9a\x80 a")), opa_string_terminated("a")) == 0);
    test("trim_space/oghamenspace_a", opa_value_compare(opa_strings_trim_space(opa_string_terminated("\xe1\x9a\x80\xe2\x80\x82 a")), opa_string_terminated("a")) == 0);
    test("trim_space/a_oghamenspace_a", opa_value_compare(opa_strings_trim_space(opa_string_terminated("a \xe1\x9a\x80\xe2\x80\x82")), opa_string_terminated("a")) == 0);

    test("lower/_", opa_value_compare(opa_strings_lower(opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("lower/a", opa_value_compare(opa_strings_lower(opa_string_terminated("a")), opa_string_terminated("a")) == 0);
    test("lower/A", opa_value_compare(opa_strings_lower(opa_string_terminated("A")), opa_string_terminated("a")) == 0);
    test("lower/AbCd", opa_value_compare(opa_strings_lower(opa_string_terminated("AbCd")), opa_string_terminated("abcd")) == 0);
    test("lower/utf-8", opa_value_compare(opa_strings_lower(opa_string_terminated("\xc4\x80")), opa_string_terminated("\xc4\x81")) == 0);
    test("lower/utf-8", opa_value_compare(opa_strings_lower(opa_string_terminated("\xc9\x83")), opa_string_terminated("\xc6\x80")) == 0);

    test("upper/_", opa_value_compare(opa_strings_upper(opa_string_terminated("")), opa_string_terminated("")) == 0);
    test("upper/a", opa_value_compare(opa_strings_upper(opa_string_terminated("a")), opa_string_terminated("A")) == 0);
    test("upper/A", opa_value_compare(opa_strings_upper(opa_string_terminated("A")), opa_string_terminated("A")) == 0);
    test("upper/AbCd", opa_value_compare(opa_strings_upper(opa_string_terminated("AbCd")), opa_string_terminated("ABCD")) == 0);
    test("upper/utf-8", opa_value_compare(opa_strings_upper(opa_string_terminated("\xc4\x81")), opa_string_terminated("\xc4\x80")) == 0);
    test("upper/utf-8", opa_value_compare(opa_strings_upper(opa_string_terminated("\xc6\x80")), opa_string_terminated("\xc9\x83")) == 0);
}

WASM_EXPORT(test_numbers_range)
void test_numbers_range(void)
{
    opa_value *a = opa_number_int(10);
    opa_value *b = opa_number_int(12);

    opa_value *exp = opa_array();
    opa_array_t *arr = opa_cast_array(exp);
    opa_array_append(arr, opa_number_int(10));
    opa_array_append(arr, opa_number_int(11));
    opa_array_append(arr, opa_number_int(12));

    test("number.range/ascending", opa_value_compare(opa_numbers_range(a, b), exp) == 0);

    opa_value *reversed = opa_array();
    arr = opa_cast_array(reversed);
    opa_array_append(arr, opa_number_int(12));
    opa_array_append(arr, opa_number_int(11));
    opa_array_append(arr, opa_number_int(10));

    test("numbers.range/descending", opa_value_compare(opa_numbers_range(b, a), reversed) == 0);
    test("numbers.range/bad operand", opa_numbers_range(opa_string_terminated("foo"), opa_number_int(10)) == NULL);
    test("numbers.range/bad operand", opa_numbers_range(opa_number_int(10), opa_string_terminated("foo")) == NULL);
}

WASM_EXPORT(test_to_number)
void test_to_number(void)
{
    test("to_number/null", opa_value_compare(opa_to_number(opa_null()), opa_number_int(0)) == 0);
    test("to_number/false", opa_value_compare(opa_to_number(opa_boolean(false)), opa_number_int(0)) == 0);
    test("to_number/true", opa_value_compare(opa_to_number(opa_boolean(true)), opa_number_int(1)) == 0);
    test("to_number/nop", opa_value_compare(opa_to_number(opa_number_int(1)), opa_number_int(1)) == 0);
    test("to_number/integer", opa_value_compare(opa_to_number(opa_string_terminated("10")), opa_number_int(10)) == 0);
    test("to_number/float", opa_value_compare(opa_to_number(opa_string_terminated("3.5")), opa_number_float(3.5)) == 0);
    test("to_number/bad string", opa_to_number(opa_string_terminated("deadbeef")) == NULL);
    test("to_number/bad value", opa_to_number(opa_array()) == NULL);
}

WASM_EXPORT(test_cidr_contains)
void test_cidr_contains(void)
{
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("10.0.0.0/8"), opa_string_terminated("10.1.0.0/24")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("172.17.0.0/24"), opa_string_terminated("172.17.0.0/16")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("10.0.0.0/8"), opa_string_terminated("192.168.1.0/24")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("10.0.0.0/8"), opa_string_terminated("10.1.1.1/32")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("2001:4860:4860::8888/32"), opa_string_terminated("2001:4860:4860:1234::8888/40")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("2001:4860:4860::8888/32"), opa_string_terminated("2001:4860:4860:1234:5678:1234:5678:8888/128")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("2001:4860::/96"), opa_string_terminated("2001:4860::/32")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("2001:4860::/32"), opa_string_terminated("fd1e:5bfe:8af3:9ddc::/64")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_cidr_contains(opa_string_terminated("not-a-cidr"), opa_string_terminated("192.168.1.67")) == NULL);
    test("cidr/contains", opa_cidr_contains(opa_string_terminated("192.168.1.0/28"), opa_string_terminated("not-a-cidr")) == NULL);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("10.0.0.0/8"), opa_string_terminated("10.1.2.3")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_contains(opa_string_terminated("10.0.0.0/8"), opa_string_terminated("192.168.1.1")), opa_boolean(false)) == 0);

    test("cidr/contains", opa_value_compare(opa_cidr_intersects(opa_string_terminated("192.168.1.0/25"), opa_string_terminated("192.168.1.64/25")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_intersects(opa_string_terminated("192.168.1.0/24"), opa_string_terminated("192.168.2.0/24")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_intersects(opa_string_terminated("fd1e:5bfe:8af3:9ddc::/64"), opa_string_terminated("fd1e:5bfe:8af3:9ddc:1111::/72")), opa_boolean(true)) == 0);
    test("cidr/contains", opa_value_compare(opa_cidr_intersects(opa_string_terminated("fd1e:5bfe:8af3:9ddc::/64"), opa_string_terminated("2001:4860:4860::8888/32")), opa_boolean(false)) == 0);
    test("cidr/contains", opa_cidr_intersects(opa_string_terminated("not-a-cidr"), opa_string_terminated("192.168.1.0/24")) == NULL);
    test("cidr/contains", opa_cidr_intersects(opa_string_terminated("192.168.1.0/28"), opa_string_terminated("not-a-cidr")) == NULL);
}

opa_value *__new_value_path(int sz, ...)
{
    va_list ap;
    opa_value *path = opa_array();

    va_start(ap, sz);

    for (int i = 0; i < sz; i++)
    {
        const char* p = va_arg(ap, const char*);
        opa_array_append(opa_cast_array(path), opa_string_terminated(p));
    }

    va_end(ap);

    return path;
}

WASM_EXPORT(test_opa_value_add_path)
void test_opa_value_add_path() {
    opa_value *data = opa_object();
    opa_value *update = opa_object();
    opa_value *path;
    opa_errc rc;

    opa_object_insert(opa_cast_object(update), opa_string_terminated("a"), opa_number_int(1));

    rc = opa_value_add_path(data, opa_array(), update);
    test_errc_eq("empty_path_rc", OPA_ERR_INVALID_PATH, rc);

    // Setup base document
    data = opa_object();
    opa_object_insert(opa_cast_object(data), opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(opa_cast_object(data), opa_string_terminated("c"), opa_number_int(3));
    opa_object_insert(opa_cast_object(data), opa_string_terminated("d"), opa_number_int(4));

    // Upsert into existing object path
    update = opa_object();
    opa_object_insert(opa_cast_object(update), opa_string_terminated("x"), opa_number_int(5));

    path = __new_value_path(1, "b");
    rc = opa_value_add_path(data, path, update);
    test_errc_eq("overwrite_sub_path_rc", OPA_ERR_OK, rc);
    test_str_eq("overwrite_sub_path", "{\"d\":4,\"c\":3,\"b\":{\"x\":5}}", opa_json_dump(data))

    // Upsert w/ creating nested path
    update = opa_object();
    opa_object_insert(opa_cast_object(update), opa_string_terminated("foo"), opa_number_int(123));

    path = __new_value_path(5, "b", "y", "z", "p", "q");
    rc = opa_value_add_path(data, path, update);
    test_errc_eq("mkdir_rc", OPA_ERR_OK, rc);
    char *exp = "{\"d\":4,\"c\":3,\"b\":{\"y\":{\"z\":{\"p\":{\"q\":{\"foo\":123}}}},\"x\":5}}";
    test_str_eq("mkdir", exp, opa_json_dump(data));

    // NULL path
    update = opa_object();
    rc = opa_value_add_path(update, NULL, opa_object());
    test_errc_eq("null_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("null_path_unchanged", exp, opa_json_dump(data));

    // non-array path types
    rc = opa_value_add_path(update, opa_string_terminated("foo"), opa_object());
    test_errc_eq("invalid_string_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("invalid_string_path_unchanged", exp, opa_json_dump(data));

    rc = opa_value_add_path(update, opa_number_int(1), opa_object());
    test_errc_eq("invalid_number_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("invalid_number_path_unchanged", exp, opa_json_dump(data));

    rc = opa_value_add_path(update, opa_set(), opa_object());
    test_errc_eq("invalid_set_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("invalid_set_path_unchanged", exp, opa_json_dump(data));

    // invalid nested object type
    opa_value *base = opa_object();
    opa_value *invalid_node = opa_set();
    opa_set_add(opa_cast_set(invalid_node), opa_string_terminated("y"));
    opa_object_insert(opa_cast_object(base), opa_string_terminated("x"), invalid_node);

    exp = "{\"x\":[\"y\"]}";
    path = __new_value_path(3, "x", "y", "z");

    rc = opa_value_add_path(base, path, opa_object());
    test_errc_eq("invalid_nested_object_in_path_rc", OPA_ERR_INVALID_TYPE, rc);
    test_str_eq("invalid_nested_object_in_path_unchanged", exp, opa_json_dump(base));

    // invalid nested object type leaf
    base = opa_object();
    opa_object_insert(opa_cast_object(base), opa_string_terminated("x"), opa_number_int(5));
    exp = "{\"x\":5}";
    path = __new_value_path(3, "x", "y", "z");

    rc = opa_value_add_path(base, path, opa_object());
    test_errc_eq("invalid_nested_object_at_path_end_rc", OPA_ERR_INVALID_TYPE, rc);
    test_str_eq("invalid_nested_object_at_path_end_unchanged", exp, opa_json_dump(base));
}

WASM_EXPORT(test_opa_object_delete)
void test_opa_object_delete(void)
{
    opa_value *data = opa_object();

    opa_object_insert(opa_cast_object(data), opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(opa_cast_object(data), opa_string_terminated("b"), opa_number_int(2));
    opa_object_insert(opa_cast_object(data), opa_string_terminated("c"), opa_number_int(3));

    opa_object_remove(opa_cast_object(data), opa_string_terminated("a"));
    test_str_eq("remove_key", "{\"c\":3,\"b\":2}", opa_json_dump(data));

    opa_object_remove(opa_cast_object(data), opa_string_terminated("bad key"));
    test_str_eq("remove_unknown_key", "{\"c\":3,\"b\":2}", opa_json_dump(data));

    opa_object_remove(opa_cast_object(data), opa_string_terminated("b"));
    opa_object_remove(opa_cast_object(data), opa_string_terminated("c"));
    test_str_eq("remove_all_keys", "{}", opa_json_dump(data));

    opa_object_remove(opa_cast_object(data), opa_string_terminated("bad key"));
    test_str_eq("remove_on_empty_obj", "{}", opa_json_dump(data));
}

WASM_EXPORT(test_opa_value_remove_path)
void test_opa_value_remove_path(void)
{
    opa_value *path;
    opa_errc rc;

    char *raw = "{\"a\":{\"b\":{\"c\":{\"d\":123}}},\"x\":[1,{\"y\":{\"z\":{}}}]}";
    opa_value *data = opa_json_parse(raw, strlen(raw));

    path = opa_array();
    rc = opa_value_remove_path(data, path);
    test_errc_eq("empty_path", OPA_ERR_INVALID_PATH, rc);

    // Reset back to full data doc
    data = opa_json_parse(raw, strlen(raw));

    path = __new_value_path(1, "foo");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("path_doesnt_exist_rc", OPA_ERR_OK, rc);
    test_str_eq("path_doesnt_exist", raw, opa_json_dump(data));

    path = __new_value_path(3, "a", "b", "foo");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("path_doesnt_exist_nested_rc", OPA_ERR_OK, rc);
    test_str_eq("path_doesnt_exist_nested", raw, opa_json_dump(data));

    path = __new_value_path(4, "a", "b", "c", "d");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("leaf_rc", OPA_ERR_OK, rc);
    test_str_eq("leaf", "{\"a\":{\"b\":{\"c\":{}}},\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));

    path = __new_value_path(2, "a", "b");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("branch_rc", OPA_ERR_OK, rc);
    test_str_eq("branch", "{\"a\":{},\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));

    path = __new_value_path(1, "a");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("branch_root_rc", OPA_ERR_OK, rc);
    test_str_eq("branch_root", "{\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));

    path = __new_value_path(3, "x", "1", "y");
    rc = opa_value_remove_path(data, path);
    test_errc_eq("invalid_array_path_rc", OPA_ERR_OK, rc);
    test_str_eq("invalid_array_path", "{\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));

    path = opa_array();
    opa_array_append(opa_cast_array(path), opa_string_terminated("x"));
    opa_array_append(opa_cast_array(path), opa_number_int(1));
    opa_array_append(opa_cast_array(path), opa_string_terminated("y"));
    rc = opa_value_remove_path(data, path);
    test_errc_eq("invalid_array_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("array_index_path", "{\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));

    rc = opa_value_remove_path(opa_object(), NULL);
    test_errc_eq("null_path_rc", OPA_ERR_INVALID_PATH, rc);
    test_str_eq("array_index_path", "{\"x\":[1,{\"y\":{\"z\":{}}}]}", opa_json_dump(data));
}

#define ARGS_N(...) ((int)(sizeof((int[]){ __VA_ARGS__ })/sizeof(int)))
#define BUILD(N, ...) build(N, ARGS_N(__VA_ARGS__), __VA_ARGS__)

typedef struct {
    int length;
    int *positions;
} sequence;

typedef struct {
    int n;
    sequence *sequences;
} sequences;

static sequences* build(int n, int args, ...)
{
    va_list valist;
    va_start(valist, args);

    sequences *s = malloc(sizeof(sequences));
    s->n = n;
    s->sequences = malloc(n * sizeof(sequence));

    for (int i = 0; i < n; i++) {
        int run_length = args / n;

        int *positions = s->sequences[i].positions = malloc(sizeof(int) * run_length);
        int j = 0;
        for (; j < run_length; j++)
        {
            positions[j] = va_arg(valist, int);
        }
        s->sequences[i].length = j;
    }

    va_end(valist);
    return s;
}

static void test_submatch_string(const char *s, sequence *seq, opa_value *result)
{
    for (int i = 0; i < seq->length; i += 2)
    {
        int start = seq->positions[i];
        int end = seq->positions[i+1];

        opa_string_t *match = opa_cast_string(opa_value_get(result, opa_number_int(i/2)));
        test("regex/find_all_string_submatch", opa_strncmp(&s[start], match->v, end-start) == 0);
    }
}

WASM_EXPORT(test_regex)
void test_regex(void)
{
    test("regex/is_valid", opa_value_compare(opa_regex_is_valid(opa_string_terminated(".*")), opa_boolean(true)) == 0);
    test("regex/is_valid_non_string", opa_value_compare(opa_regex_is_valid(opa_number_int(123)), opa_boolean(false)) == 0);

    typedef struct {
        const char *pat;
        const char *text;
        sequences *sequences;
    } testcase;

    // golang find test cases from https://golang.org/src/regexp/find_test.go
    testcase tests[] = {
        {"", "", BUILD(1, 0, 0)},
        {"^abcdefg", "abcdefg", BUILD(1, 0, 7)},
        {"a+", "baaab", BUILD(1, 1, 4)},
        {"abcd..", "abcdef", BUILD(1, 0, 6)},
        {"a", "a", BUILD(1, 0, 1)},
        {"x", "y", NULL},
        {"b", "abc", BUILD(1, 1, 2)},
        {".", "a", BUILD(1, 0, 1)},
        {".*", "abcdef", BUILD(1, 0, 6)},
        {"^a", "abcde", BUILD(1, 0, 1)},
        {"^", "abcde", BUILD(1, 0, 0)},
        {"$", "abcde", BUILD(1, 5, 5)},
        {"^abcd$", "abcd", BUILD(1, 0, 4)},
        {"^bcd'", "abcdef", NULL},
        {"^abcd$", "abcde", NULL},
        {"a+", "baaab", BUILD(1, 1, 4)},
        {"a*", "baaab", BUILD(3, 0, 0, 1, 4, 5, 5)},
        {"[a-z]+", "abcd", BUILD(1, 0, 4)},
        {"[^a-z]+", "ab1234cd", BUILD(1, 2, 6)},
        {"[a\\-\\]z]+", "az]-bcz", BUILD(2, 0, 4, 6, 7)},
        {"[^\\n]+", "abcd\n", BUILD(1, 0, 4)},
        {"[日本語]+", "日本語日本語", BUILD(1, 0, 18)},
        {"日本語+", "日本語", BUILD(1, 0, 9)},
        {"日本語+", "日本語語語語", BUILD(1, 0, 18)},
        {"()", "", BUILD(1, 0, 0, 0, 0)},
        {"(a)", "a", BUILD(1, 0, 1, 0, 1)},
        {"(.)(.)", "日a", BUILD(1, 0, 4, 0, 3, 3, 4)},
        {"(.*)", "", BUILD(1, 0, 0, 0, 0)},
        {"(.*)", "abcd", BUILD(1, 0, 4, 0, 4)},
        {"(..)(..)", "abcd", BUILD(1, 0, 4, 0, 2, 2, 4)},
        {"(([^xyz]*)(d))", "abcd", BUILD(1, 0, 4, 0, 4, 0, 3, 3, 4)},
        {"((a|b|c)*(d))", "abcd", BUILD(1, 0, 4, 0, 4, 2, 3, 3, 4)},
        {"(((a|b|c)*)(d))", "abcd", BUILD(1, 0, 4, 0, 4, 0, 3, 2, 3, 3, 4)},
        {"\\a\\f\\n\\r\\t\\v", "\a\f\n\r\t\v", BUILD(1, 0, 6)},
        {"[\\a\\f\\n\\r\\t\\v]+", "\a\f\n\r\t\v", BUILD(1, 0, 6)},

        {"a*(|(b))c*", "aacc", BUILD(1, 0, 4, 2, 2, -1, -1)},
        {"(.*).*", "ab", BUILD(1, 0, 2, 0, 2)},
        {"[.]", ".", BUILD(1, 0, 1)},
        {"/$", "/abc/", BUILD(1, 4, 5)},
        {"/$", "/abc", NULL},

        // multiple matches
        {".", "abc", BUILD(3, 0, 1, 1, 2, 2, 3)},
        {"(.)", "abc", BUILD(3, 0, 1, 0, 1, 1, 2, 1, 2, 2, 3, 2, 3)},
        {".(.)", "abcd", BUILD(2, 0, 2, 1, 2, 2, 4, 3, 4)},
        {"ab*", "abbaab", BUILD(3, 0, 3, 3, 4, 4, 6)},
        {"a(b*)", "abbaab", BUILD(3, 0, 3, 1, 3, 3, 4, 4, 4, 4, 6, 5, 6)},

        // fixed bugs
        {"ab$", "cab", BUILD(1, 1, 3)},
        {"axxb$", "axxcb", NULL},
        {"data", "daXY data", BUILD(1, 5, 9)},
        {"da(.)a$", "daXY data", BUILD(1, 5, 9, 7, 8)},
        {"zx+", "zzx", BUILD(1, 1, 3)},
        {"ab$", "abcab", BUILD(1, 3, 5)},
        {"(aa)*$", "a", BUILD(1, 1, 1, -1, -1)},
        {"(?:.|(?:.a))", "", NULL},
        {"(?:A(?:A|a))", "Aa", BUILD(1, 0, 2)},
        {"(?:A|(?:A|a))", "a", BUILD(1, 0, 1)},
        {"(a){0}", "", BUILD(1, 0, 0, -1, -1)},
        {"(?-s)(?:(?:^).)", "\n", NULL},
        {"(?s)(?:(?:^).)", "\n", BUILD(1, 0, 1)},
        {"(?:(?:^).)", "\n", NULL},
        {"\\b", "x", BUILD(2, 0, 0, 1, 1)},
        {"\\b", "xx", BUILD(2, 0, 0, 2, 2)},
        {"\\b", "x y", BUILD(4, 0, 0, 1, 1, 2, 2, 3, 3)},
        {"\\b", "xx yy", BUILD(4, 0, 0, 2, 2, 3, 3, 5, 5)},
        {"\\B", "x", NULL},
        {"\\B", "xx", BUILD(1, 1, 1)},
        {"\\B", "x y", NULL},
        {"\\B", "xx yy", BUILD(2, 1, 1, 4, 4)},

        // RE2 tests
        {"[^\\S\\s]", "abcd", NULL},
        {"[^\\S[:space:]]", "abcd", NULL},
        {"[^\\D\\d]", "abcd", NULL},
        {"[^\\D[:digit:]]", "abcd", NULL},
        {"(?i)\\W", "x", NULL},
        {"(?i)\\W", "k", NULL},
        {"(?i)\\W", "s", NULL},

        // long set of matches (longer than startSize)
        {
		".",
		"qwertyuiopasdfghjklzxcvbnm1234567890",
		BUILD(36, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10,
              10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18, 19, 19, 20,
              20, 21, 21, 22, 22, 23, 23, 24, 24, 25, 25, 26, 26, 27, 27, 28, 28, 29, 29, 30,
              30, 31, 31, 32, 32, 33, 33, 34, 34, 35, 35, 36),
        },
    };

    for (int i = 0; i < sizeof(tests) / sizeof(testcase); i++)
    {
        test("regex/match", opa_value_compare(opa_regex_match(opa_string_terminated(tests[i].pat), opa_string_terminated(tests[i].text)), opa_boolean(tests[i].sequences != NULL)) == 0);
    }

    for (int i = 0; i < sizeof(tests) / sizeof(testcase); i++)
    {
        opa_value *result = opa_regex_find_all_string_submatch(opa_string_terminated(tests[i].pat), opa_string_terminated(tests[i].text), opa_number_int(-1));
        opa_array_t *arr = opa_cast_array(result);

        if (tests[i].sequences == NULL)
        {
            test("regex/find_all_string_submatch (len)", arr->len == 0);
            continue;
        }

        test("regex/find_all_string_submatch (len)", arr->len == tests[i].sequences->n);
        for (int j = 0; j < tests[i].sequences->n; j++)
        {
            test_submatch_string(tests[i].text, &tests[i].sequences->sequences[j], opa_value_get(result, opa_number_int(j)));
        }
    }
}

WASM_EXPORT(test_opa_lookup)
void test_opa_lookup(void)
{
    opa_array_t *path1 = opa_cast_array(opa_array());
    opa_array_append(path1, opa_string_terminated("foo"));
    opa_array_append(path1, opa_string_terminated("bar"));
    opa_array_append(path1, opa_string_terminated("baz"));

    opa_object_t *mock_mapping = opa_cast_object(opa_object());
    opa_object_t *obj1 = opa_cast_object(opa_object());
    opa_object_insert(obj1, opa_string_terminated("baz"), opa_number_int(1));
    opa_object_t *obj2 = opa_cast_object(opa_object());
    opa_object_insert(obj2, opa_string_terminated("bar"), &obj1->hdr);
    opa_object_insert(mock_mapping, opa_string_terminated("foo"), &obj2->hdr);

    opa_value *empty_mapping = opa_object();

    opa_object_t *smaller_mapping = opa_cast_object(opa_object());
    opa_object_t *obj3 = opa_cast_object(opa_object());
    opa_object_insert(obj3, opa_string_terminated("bar"), opa_number_int(2));
    opa_object_insert(smaller_mapping, opa_string_terminated("foo"), &obj3->hdr);

    test("opa_lookup/hit", opa_lookup(&mock_mapping->hdr, &path1->hdr) == 1);
    test("opa_lookup/miss", opa_lookup(empty_mapping, &path1->hdr) == 0);
    test("opa_lookup/miss/less", opa_lookup(&smaller_mapping->hdr, &path1->hdr) == 0);
}

WASM_EXPORT(test_opa_mapping_init)
void test_opa_mapping_init(void)
{
    opa_string_t *s = opa_cast_string(opa_string_terminated("{\"foo\": {\"bar\": 123}}"));
    opa_mapping_init(s->v, s->len);

    opa_array_t *path1 = opa_cast_array(opa_array());
    opa_array_append(path1, opa_string_terminated("foo"));
    opa_array_append(path1, opa_string_terminated("bar"));
    test("opa_mapping_init/opa_lookup_works", opa_mapping_lookup(&path1->hdr) == 123);
}
