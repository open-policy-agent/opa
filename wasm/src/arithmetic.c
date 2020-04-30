#include <mpdecimal.h>
#include <stdio.h>

#include "str.h"
#include "value.h"
#include "set.h"

static int initialized;
static mpd_context_t ctx;

mpd_context_t *mpd_ctx(void)
{
    if (!initialized)
    {
        mpd_defaultcontext(&ctx);
        ctx.traps = 0;
    }

    return &ctx;
}

mpd_t *opa_number_to_bf(opa_value *v)
{
    if (opa_value_type(v) != OPA_NUMBER)
    {
        return NULL;
    }

    opa_number_t *n = opa_cast_number(v);
    mpd_t *r = NULL;
    uint32_t status = 0;

    switch (n->repr)
    {
    case OPA_NUMBER_REPR_FLOAT:
        {
            char buf[32]; // PRINTF_FTOA_BUFFER_SIZE
            if (snprintf(buf, sizeof(buf), "%f", n->v.f) == sizeof(buf))
            {
                opa_abort("opa_number_to_bf: overflow");
            }

            r = mpd_qnew();
            mpd_qset_string(r, buf, mpd_ctx(), &status);
        }
        break;

    case OPA_NUMBER_REPR_REF:
        r = mpd_qnew();
        mpd_qset_string(r, n->v.ref.s, mpd_ctx(), &status);

        if (status != 0)
        {
            opa_abort("opa_number_to_bf: invalid number");
        }
        break;

    case OPA_NUMBER_REPR_INT:
        r = mpd_qnew();

        if (n->v.i >= INT32_MIN && n->v.i <= INT32_MAX)
        {
            mpd_qset_i32(r, (int32_t)n->v.i, mpd_ctx(), &status);
        } else {
            char buf[32]; // PRINTF_NTOA_BUFFER_SIZE
            if (snprintf(buf, sizeof(buf), "%d", n->v.i) == sizeof(buf))
            {
                opa_abort("opa_number_to_bf: overflow");
            }

            r = mpd_qnew();
            mpd_qset_string(r, buf, mpd_ctx(), &status);
        }
        break;

    default:
        opa_abort("opa_number_to_bf: illegal repr");
        return NULL;
    }

    if (status != 0)
    {
        opa_abort("opa_number_to_bf: invalid number x");
    }

    return r;
}

opa_value *opa_bf_to_number(mpd_t* n)
{
    uint32_t status = 0;
    int32_t i = mpd_qget_i32(n, &status);

    if (status == 0)
    {
        return opa_number_int(i);
    }

    char *s = mpd_to_sci(n, 0);
    return opa_number_ref(s, opa_strlen(s));
}

opa_value *opa_arith_abs(opa_value *v)
{
    mpd_t *n = opa_number_to_bf(v);
    if (n == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qabs(r, n, mpd_ctx(), &status);
    mpd_del(n);

    if (status != 0)
    {
        opa_abort("opa_number_to_bf: invalid number");
    }

    v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
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

    mpd_qround_to_int(r, n, mpd_ctx(), &status);
    mpd_del(n);

    if (status != 0)
    {
        opa_abort("opa_arith_round: invalid number");
    }

    v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
}

opa_value *opa_arith_plus(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qadd(r, x, y, mpd_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status != 0)
    {
        opa_abort("opa_arith_plus: invalid number");
    }

    opa_value *v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
}

opa_value *opa_arith_minus(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x != NULL && y != NULL)
    {
        mpd_t *r = mpd_qnew();
        uint32_t status = 0;

        mpd_qsub(r, x, y, mpd_ctx(), &status);
        mpd_del(x);
        mpd_del(y);

        if (status != 0)
        {
            opa_abort("opa_arith_minus: invalid number");
        }

        opa_value *v = opa_bf_to_number(r);
        mpd_del(r);
        return v;
    }

    return opa_set_diff(a, b);
}

opa_value *opa_arith_multiply(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qmul(r, x, y, mpd_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status != 0)
    {
        opa_abort("opa_arith_multiply: invalid number");
    }

    opa_value *v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
}

opa_value *opa_arith_divide(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qdiv(r, x, y, mpd_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status & MPD_Division_by_zero)
    {
        opa_abort("opa_arith_divide: divide by zero"); // TODO: Report error instead.
    }

    if (status != 0)
    {
        opa_abort("opa_arith_divide: invalid number");
    }

    opa_value *v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
}

opa_value *opa_arith_rem(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qrem(r, x, y, mpd_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status)
    {
        opa_abort("opa_arith_rem: non-integer remainder"); // TODO: Report error instead.
    }

    opa_value *v = opa_bf_to_number(r);
    mpd_del(r);
    return v;
}
