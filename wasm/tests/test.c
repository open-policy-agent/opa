#include "string.h"
#include "json.h"

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
    test_lex_buffer("escaped quote", "\"\\\"\"", "\\\"", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped reverse solidus", "\"\\\\\"", "\\\\", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped solidus", "\"\\/\"", "\\/", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped backspace", "\"\\b\"", "\\b", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped feed forward", "\"\\f\"", "\\f", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped line feed", "\"\\n\"", "\\n", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped carriage return", "\"\\r\"", "\\r", OPA_JSON_TOKEN_STRING);
    test_lex_buffer("escaped tab", "\"\\t\"", "\\t", OPA_JSON_TOKEN_STRING);
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

void test_opa_value_length()
{
    opa_array_t *arr = fixture_array1();
    opa_object_t *obj = fixture_object1();

    test("arrays", opa_value_length(&arr->hdr) == 4);
    test("objects", opa_value_length(&obj->hdr) == 2);
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
}

void test_opa_value_iter_object()
{
    opa_object_t *obj = fixture_object1();

    opa_value *k1 = opa_value_iter(&obj->hdr, NULL);
    opa_value *k2 = opa_value_iter(&obj->hdr, k1);
    opa_value *k3 = opa_value_iter(&obj->hdr, k2);

    opa_value *exp1 = opa_string_terminated("a");
    opa_value *exp2 = opa_string_terminated("b");
    opa_value *exp3 = NULL;

    if (opa_value_not_equal(k1, exp1))
    {
        test_fatal("object iter start did not return expected value");
    }

    if (opa_value_not_equal(k2, exp2))
    {
        test_fatal("object iter second did not return expected value");
    }

    if (opa_value_not_equal(k3, exp3))
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

    if (opa_value_not_equal(k1, exp1))
    {
        test_fatal("array iter start did not return expected value");
    }

    if (opa_value_not_equal(k2, exp2))
    {
        test_fatal("array iter second did not return expected value");
    }

    if (opa_value_not_equal(k3, exp3))
    {
        test_fatal("array iter third did not return expected value");
    }
}
