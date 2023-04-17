#include <locale.h>

static struct lconv lc;

/* POSIX/C locale defaults. */

struct lconv *localeconv(void)
{
    lc.decimal_point = ".";
    lc.thousands_sep = "";
    lc.grouping = "-1";
    return &lc;
}
