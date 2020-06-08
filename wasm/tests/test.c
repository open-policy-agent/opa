#include <ctype.h>

#include "aggregates.h"
#include "arithmetic.h"
#include "array.h"
#include "bits-builtins.h"
#include "json.h"
#include "malloc.h"
#include "mpd.h"
#include "set.h"
#include "str.h"
#include "types.h"

void opa_test_fail(const char *note, const char *func, const char *file, int line);
void opa_test_pass(const char *note, const char *func);

#define test_fatal(note)                                   \
    {                                                      \
        opa_test_fail(note, __func__, __FILE__, __LINE__); \
        return;                                            \
    }

#define test(note, expr)                                   \
    if (!(expr))                                           \
    {                                                      \
        opa_test_fail(note, __func__, __FILE__, __LINE__); \
    }                                                      \
    else                                                   \
    {                                                      \
        opa_test_pass(note, __func__);                     \
    }

void test_opa_malloc()
{
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

void test_opa_free()
{

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

    p1 = opa_malloc(64);
    p2 = opa_malloc(64);
    high = opa_heap_ptr_get();

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 1);

    p1 = opa_malloc(1);
    test("free blocks", opa_heap_free_blocks() == 1);
    test("heap ptr", high == opa_heap_ptr_get());

    opa_free(p2);
    test("free blocks", opa_heap_free_blocks() == 0);

    opa_free(p1);
    test("free blocks", opa_heap_free_blocks() == 0);
    test("heap ptr", base == opa_heap_ptr_get());
}

void test_opa_strlen()
{
    test("empty", opa_strlen("") == 0);
    test("non-empty", opa_strlen("1234") == 4);
}

void test_opa_strncmp()
{
    test("empty", opa_strncmp("", "", 0) == 0);
    test("equal", opa_strncmp("1234", "1234", 4) == 0);
    test("less than", opa_strncmp("1234", "1243", 4) < 0);
    test("greater than", opa_strncmp("1243", "1234", 4) > 0);
}

void test_opa_strcmp()
{
    test("empty", opa_strcmp("", "") == 0);
    test("equal", opa_strcmp("abcd", "abcd") == 0);
    test("less than", opa_strcmp("1234", "1243") < 0);
    test("greater than", opa_strcmp("1243", "1234") > 0);
    test("shorter", opa_strcmp("123", "1234") < 0);
    test("longer", opa_strcmp("1234", "123") > 0);
}

void test_opa_itoa()
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

void test_opa_atoi64()
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

void test_opa_atof64()
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

int lex_crunch(const char *s)
{
    opa_json_lex ctx;
    opa_json_lex_init(s, opa_strlen(s), &ctx);
    return opa_json_lex_read(&ctx);
}

void test_opa_lex_tokens()
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

void test_opa_lex_buffer()
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

void test_opa_value_compare()
{
    test("none", opa_value_compare(NULL, NULL) == 0);
    test("none/some", opa_value_compare(NULL, opa_null()) < 0);
    test("some/none", opa_value_compare(opa_null(), NULL) > 0);
    test("null", opa_value_compare(opa_null(), opa_null()) == 0);
    test("null/boolean", opa_value_compare(opa_boolean(TRUE), opa_null()) > 0);
    test("true/true", opa_value_compare(opa_boolean(TRUE), opa_boolean(TRUE)) == 0);
    test("true/false", opa_value_compare(opa_boolean(TRUE), opa_boolean(FALSE)) > 0);
    test("false/true", opa_value_compare(opa_boolean(FALSE), opa_boolean(TRUE)) < 0);
    test("false/false", opa_value_compare(opa_boolean(FALSE), opa_boolean(FALSE)) == 0);
    test("number/boolean", opa_value_compare(opa_number_int(100), opa_boolean(TRUE)) > 0);
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

void test_opa_json_parse_scalar()
{
    test("null", parse_crunch("null", opa_null()));
    test("true", parse_crunch("true", opa_boolean(TRUE)));
    test("false", parse_crunch("false", opa_boolean(FALSE)));
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

void test_opa_json_max_str_len()
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

opa_array_t *fixture_array1()
{
    opa_array_t *arr = opa_cast_array(opa_array());
    opa_array_append(arr, opa_number_int(1));
    opa_array_append(arr, opa_number_int(2));
    opa_array_append(arr, opa_number_int(3));
    opa_array_append(arr, opa_number_int(4));
    return arr;
}

opa_array_t *fixture_array2()
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

opa_object_t *fixture_object1()
{
    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, opa_string_terminated("a"), opa_number_int(1));
    opa_object_insert(obj, opa_string_terminated("b"), opa_number_int(2));
    return obj;
}

opa_object_t *fixture_object2()
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

opa_set_t *fixture_set1()
{
    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, opa_string_terminated("a"));
    opa_set_add(set, opa_string_terminated("b"));
    return set;
}

void test_opa_value_length()
{
    opa_array_t *arr = fixture_array1();
    opa_object_t *obj = fixture_object1();
    opa_set_t *set = fixture_set1();

    test("arrays", opa_value_length(&arr->hdr) == 4);
    test("objects", opa_value_length(&obj->hdr) == 2);
    test("sets", opa_value_length(&set->hdr) == 2);
}

void test_opa_value_get_array()
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

void test_opa_array_sort()
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

void test_opa_value_get_object()
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

void test_opa_json_parse_composites()
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

void test_opa_object_insert()
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

void test_opa_object_growth()
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

void test_opa_set_add_and_get()
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

void test_opa_set_growth()
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

void test_opa_value_iter_object()
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

void test_opa_value_iter_array()
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

void test_opa_value_iter_set()
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

void test_opa_value_merge_fail()
{
    opa_value *fail = opa_value_merge(opa_number_int(1), opa_string_terminated("foo"));

    if (fail != NULL)
    {
        test_fatal("expected merge of two scalars to fail");
    }
}

void test_opa_value_merge_simple()
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


void test_opa_value_merge_nested()
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

void test_opa_value_shallow_copy()
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

void test_opa_json_dump()
{
    test("null", opa_strcmp(opa_json_dump(opa_null()), "null") == 0);
    test("false", opa_strcmp(opa_json_dump(opa_boolean(0)), "false") == 0);
    test("true", opa_strcmp(opa_json_dump(opa_boolean(1)), "true") == 0);
    test("strings", opa_strcmp(opa_json_dump(opa_string_terminated("hello\"world")), "\"hello\\\"world\"") == 0);
    test("strings utf-8", opa_strcmp(opa_json_dump(opa_string_terminated("\xed\xba\xad")), "\"\xed\xba\xad\"") == 0);
    test("numbers", opa_strcmp(opa_json_dump(opa_number_int(127)), "127") == 0);

    // NOTE(tsandall): the string representation is lossy. We should store
    // user-supplied floating-point values as strings so that round-trip
    // operations are lossless. Computed values can be lossy for the time being.
    test("numbers/float", opa_strcmp(opa_json_dump(opa_number_float(12345.678)), "12345.7") == 0);

    // NOTE(tsandall): trailing zeros should be omitted but this appears to be an open issue: https://github.com/mpaland/printf/issues/55
    test("numbers/float", opa_strcmp(opa_json_dump(opa_number_float(10.5)), "10.5000") == 0);

    test("numbers/ref", opa_strcmp(opa_json_dump(opa_number_ref("127", 3)), "127") == 0);

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
    opa_array_append(opa_cast_array(terminators), opa_boolean(1));
    opa_array_append(opa_cast_array(terminators), opa_boolean(0));
    opa_array_append(opa_cast_array(terminators), opa_null());

    test("bool/null terminators", opa_strcmp(opa_json_dump(terminators), "[true,false,null]") == 0);
}

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
    test("remainder 5 % 2", opa_number_as_float(opa_cast_number(opa_arith_rem(opa_number_float(5), opa_number_float(2)))) == 1);
}

void test_set_diff(void)
{
    // test_arithmetic covers the diff.
}

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
}

void test_types(void)
{
    test("is_number", opa_value_compare(opa_types_is_number(opa_number_int(0)), opa_boolean(true)) == 0);
    test("is_number", opa_types_is_number(opa_null()) == NULL);
    test("is_string", opa_value_compare(opa_types_is_string(opa_string("a", 1)), opa_boolean(true)) == 0);
    test("is_string", opa_types_is_string(opa_null()) == NULL);
    test("is_boolean", opa_value_compare(opa_types_is_boolean(opa_boolean(true)), opa_boolean(true)) == 0);
    test("is_boolean", opa_types_is_boolean(opa_null()) == NULL);
    test("is_array", opa_value_compare(opa_types_is_array(opa_array()), opa_boolean(true)) == 0);
    test("is_array", opa_types_is_array(opa_null()) == NULL);
    test("is_set", opa_value_compare(opa_types_is_set(opa_set()), opa_boolean(true)) == 0);
    test("is_set", opa_types_is_set(opa_null()) == NULL);
    test("is_object", opa_value_compare(opa_types_is_object(opa_object()), opa_boolean(true)) == 0);
    test("is_object", opa_types_is_object(opa_null()) == NULL);
    test("is_null", opa_value_compare(opa_types_is_null(opa_null()), opa_boolean(true)) == 0);
    test("is_null", opa_types_is_null(opa_number_int(0)) == NULL);

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
    opa_array_append(arr_trues, opa_boolean(TRUE));
    opa_array_append(arr_trues, opa_boolean(TRUE));

    opa_array_t *arr_mixed = opa_cast_array(opa_array());
    opa_array_append(arr_mixed, opa_boolean(TRUE));
    opa_array_append(arr_mixed, opa_boolean(FALSE));

    opa_array_t *arr_falses = opa_cast_array(opa_array());
    opa_array_append(arr_falses, opa_boolean(FALSE));
    opa_array_append(arr_falses, opa_boolean(FALSE));

    test("all/array trues", opa_value_compare(opa_agg_all(&arr_trues->hdr), opa_boolean(TRUE)) == 0);
    test("all/array mixed", opa_value_compare(opa_agg_all(&arr_mixed->hdr), opa_boolean(FALSE)) == 0);
    test("all/array falses", opa_value_compare(opa_agg_all(&arr_falses->hdr), opa_boolean(FALSE)) == 0);
    test("any/array trues", opa_value_compare(opa_agg_any(&arr_trues->hdr), opa_boolean(TRUE)) == 0);
    test("any/array mixed", opa_value_compare(opa_agg_any(&arr_mixed->hdr), opa_boolean(TRUE)) == 0);
    test("any/array falses", opa_value_compare(opa_agg_any(&arr_falses->hdr), opa_boolean(FALSE)) == 0);

    opa_set_t *set_trues = opa_cast_set(opa_set());
    opa_set_add(set_trues, opa_boolean(TRUE));
    opa_set_add(set_trues, opa_boolean(TRUE));

    opa_set_t *set_mixed = opa_cast_set(opa_set());
    opa_set_add(set_mixed, opa_boolean(TRUE));
    opa_set_add(set_mixed, opa_boolean(FALSE));

    opa_set_t *set_falses = opa_cast_set(opa_set());
    opa_set_add(set_falses, opa_boolean(FALSE));
    opa_set_add(set_falses, opa_boolean(FALSE));

    test("all/set trues", opa_value_compare(opa_agg_all(&set_trues->hdr), opa_boolean(TRUE)) == 0);
    test("all/set mixed", opa_value_compare(opa_agg_all(&set_mixed->hdr), opa_boolean(FALSE)) == 0);
    test("all/set falses", opa_value_compare(opa_agg_all(&set_falses->hdr), opa_boolean(FALSE)) == 0);
    test("any/set trues", opa_value_compare(opa_agg_any(&set_trues->hdr), opa_boolean(TRUE)) == 0);
    test("any/set mixed", opa_value_compare(opa_agg_any(&set_mixed->hdr), opa_boolean(TRUE)) == 0);
    test("any/set falses", opa_value_compare(opa_agg_any(&set_falses->hdr), opa_boolean(FALSE)) == 0);
}
