#ifndef OPA_LOCALE_H
#define OPA_LOCALE_H

struct lconv
{
    char *decimal_point;
    char *thousands_sep;
    char *grouping;
};

struct lconv *localeconv(void);

#endif
