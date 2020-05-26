#include <mpdecimal.h>
#include <stdio.h>

#include "mpd.h"
#include "set.h"
#include "str.h"
#include "value.h"

opa_value *opa_arith_abs(opa_value *v)
{
    mpd_t *n = opa_number_to_bf(v);
    if (n == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qabs(r, n, mpd_max_ctx(), &status);
    mpd_del(n);

    if (status != 0)
    {
        opa_abort("opa_number_to_bf: invalid number");
    }

    return opa_bf_to_number(r);
}

opa_value *opa_arith_round(opa_value *v)
{
    mpd_t *n = opa_number_to_bf(v);
    if (n == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qround_to_int(r, n, mpd_max_ctx(), &status);
    mpd_del(n);

    if (status != 0)
    {
        opa_abort("opa_arith_round: invalid number");
    }

    return opa_bf_to_number(r);
}

opa_value *opa_arith_plus(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qadd(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    status &= ~(MPD_Rounded | MPD_Inexact);
    if (status != 0)
    {
        opa_abort("opa_arith_plus: invalid number");
    }

    return opa_bf_to_number(r);
}

opa_value *opa_arith_minus(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x != NULL && y != NULL)
    {
        mpd_t *r = mpd_qnew();
        uint32_t status = 0;

        mpd_qsub(r, x, y, mpd_max_ctx(), &status);
        mpd_del(x);
        mpd_del(y);

        status &= ~(MPD_Rounded | MPD_Inexact);
        if (status != 0)
        {
            opa_abort("opa_arith_minus: invalid number");
        }

        return opa_bf_to_number(r);
    }

    opa_mpd_del(x);
    opa_mpd_del(y);

    return opa_set_diff(a, b);
}

opa_value *opa_arith_multiply(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qmul(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    status &= ~(MPD_Rounded | MPD_Inexact);
    if (status != 0)
    {
        opa_abort("opa_arith_multiply: invalid number");
    }

    return opa_bf_to_number(r);
}

opa_value *opa_arith_divide(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    // Use the default context to enforce rounding, similar to golang.
    mpd_qdiv(r, x, y, mpd_default_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status & MPD_Division_by_zero)
    {
        opa_abort("opa_arith_divide: divide by zero"); // TODO: Report error instead.
    }

    status &= ~(MPD_Rounded | MPD_Inexact);
    if (status != 0)
    {
        opa_abort("opa_arith_divide: invalid number");
    }

    return opa_bf_to_number(r);
}

opa_value *opa_arith_rem(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qrem(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status)
    {
        opa_abort("opa_arith_rem: non-integer remainder"); // TODO: Report error instead.
    }

    return opa_bf_to_number(r);
}
