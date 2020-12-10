#include "error.h"
#include "malloc.h"
#include "mpd.h"
#include "printf.h"
#include "str.h"

void opa_runtime_error(const char *loc, int row, int col, const char *msg)
{
    char row_str[sizeof(row)*8+1];
    char col_str[sizeof(col)*8+1];
    opa_itoa(row, row_str, 10);
    opa_itoa(col, col_str, 10);
    // 5 = ":" + ":" + ": " + \0
    size_t len = opa_strlen(loc)+opa_strlen(row_str)+opa_strlen(col_str)+opa_strlen(msg)+5;
    char *err = (char *)opa_malloc(len);
    snprintf(err, len, "%s:%s:%s: %s", loc, row_str, col_str, msg);

    opa_abort(err);
}
