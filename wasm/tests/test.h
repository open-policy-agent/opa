#ifndef OPA_TEST_H
#define OPA_TEST_H

#ifdef __cplusplus
extern "C" {
#endif

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

#define FAIL_TEMPLATE_STR "%s: expected: '%s' actual: '%s'"
#define FAIL_TEMPLATE_ERRC "%s: expected error: %d actual error: %d"

#define test_with_exp(note, expr, exp, actual, template)        \
    if (expr)                                                   \
    {                                                           \
        opa_test_pass(note, __func__);                          \
    }                                                           \
    else                                                        \
    {                                                           \
        char msg[256];                                          \
        snprintf(msg, 256, template, note, exp, actual);        \
        opa_test_fail(msg, __func__, __FILE__, __LINE__);       \
    }

#define test_str_eq(note, exp, actual) test_with_exp(note, opa_strcmp(exp, actual) == 0, exp, actual, FAIL_TEMPLATE_STR)
#define test_errc_eq(note, exp, actual) test_with_exp(note, exp == actual, exp, actual, FAIL_TEMPLATE_ERRC)

#ifdef __cplusplus
}
#endif

#endif
