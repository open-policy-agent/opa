#include "string.h"
#include "json.h"
#include "malloc.h"

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
    test("integers", parse_crunch("0", opa_number_int(0)));
    test("integers", parse_crunch("123456789", opa_number_int(123456789)));
    test("signed integers", parse_crunch("-0", opa_number_int(0)));
    test("signed integers", parse_crunch("-123456789", opa_number_int(-123456789)));
    test("floats", parse_crunch("16.7", opa_number_float(16.7)));
    test("signed floats", parse_crunch("-16.7", opa_number_float(-16.7)));
    test("exponents", parse_crunch("6e7", opa_number_float(6e7)));
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

    opa_value *res = &arr->hdr;
    opa_value *exp = &fixture_array1()->hdr;

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
