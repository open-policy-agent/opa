#ifndef OPA_LOCALE_H
#define OPA_LOCALE_H

#ifdef __cplusplus
extern "C" {
#endif

struct lconv
{
    char *decimal_point;
    char *thousands_sep;
    char *grouping;
};

struct lconv *localeconv(void);

#ifdef __cplusplus
}
#endif

#endif
