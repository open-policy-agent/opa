#include "arithmetic.h"
#include "bits.h"
#include "mpd.h"
#include "std.h"

#define swap(x, y) { mpd_t *v = x; x = y; y = v; }

OPA_BUILTIN
opa_value *opa_bits_or(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL || !mpd_isinteger(x) || !mpd_isinteger(y))
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    if (mpd_sign(x) == mpd_sign(y))
    {
        if (mpd_sign(x)) // x neg
        {
            // (-x) | (-y) == ^(x-1) | ^(y-1) == ^((x-1) & (y-1)) == -(((x-1) & (y-1)) + 1)
            mpd_t *x1 = qsub_one(qabs(x));
            mpd_t *y1 = qsub_one(qabs(y));
            mpd_t *z = qadd_one(qand(x1, y1));
            return opa_bf_to_number(qneg(z));
        }

        // x | y == x | y
        return opa_bf_to_number(qor(x, y));
    }

    // x.neg != y.neg
    if (mpd_sign(x))
    {
        // | is symmetric
        swap(x, y);
    }

    // x | (-y) == x | ^(y-1) == ^((y-1) &^ x) == -(^((y-1) &^ x) + 1)
    mpd_t *y1 = qsub_one(qabs(y));
    mpd_t *z = qadd_one(qand_not(y1, x));
    return opa_bf_to_number(qneg(z));
}

OPA_BUILTIN
opa_value *opa_bits_and(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL || !mpd_isinteger(x) || !mpd_isinteger(y))
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    if (mpd_sign(x) == mpd_sign(y))
    {
        if (mpd_sign(x)) // x neg
        {
            // (-x) & (-y) == ^(x-1) & ^(y-1) == ^((x-1) | (y-1)) == -(((x-1) | (y-1)) + 1)
            mpd_t *x1 = qsub_one(qabs(x));
            mpd_t *y1 = qsub_one(qabs(y));
            mpd_t *z = qadd_one(qor(x1, y1));
            return opa_bf_to_number(qneg(z));
        }

        // x & y == x & y
        return opa_bf_to_number(qand(x, y));
    }

    // x.neg != y.neg
    if (mpd_sign(x))
    {
        // & is symmetric
        swap(x, y);
    }

    // x & (-y) == x & ^(y-1) == x &^ (y-1)
    mpd_t *y1 = qsub_one(qabs(y));
    mpd_t *z = qand_not(x, y1);
    return opa_bf_to_number(z);
}

OPA_BUILTIN
opa_value *opa_bits_negate(opa_value *a)
{
    mpd_t *x = opa_number_to_bf(a);
    if (x == NULL || !mpd_isinteger(x))
    {
        return NULL;
    }

    if (mpd_sign(x))
    {
        // ^(-x) == ^(^(x-1)) == x-1

        x = opa_bf_to_bf_bits(qabs(x));
        if (x == NULL)
        {
            // Not an integer.
            return NULL;
        }

        x = opa_bf_bits_to_bf(x);
        mpd_t *z = qsub_one(x);
        return opa_bf_to_number(z);
    }

    // ^x == -x-1 == -(x+1)
    return opa_bf_to_number(qneg(qadd_one(x)));
}

OPA_BUILTIN
opa_value *opa_bits_xor(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    mpd_t *y = opa_number_to_bf(b);
    if (x == NULL || y == NULL || !mpd_isinteger(x) || !mpd_isinteger(y))
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    if (mpd_sign(x) == mpd_sign(y))
    {
        if (mpd_sign(x)) // x neg
        {
            // (-x) ^ (-y) == ^(x-1) ^ ^(y-1) == (x-1) ^ (y-1)
            mpd_t *x1 = qsub_one(qabs(x));
            mpd_t *y1 = qsub_one(qabs(y));
            return opa_bf_to_number(qxor(x1, y1));
        }

        // x ^ y == x ^ y
        return opa_bf_to_number(qxor(x, y));
    }

    // x.neg != y.neg
    if (mpd_sign(x))
    {
        // ^ is symmetric
        swap(x, y);
    }

    // x ^ (-y) == x ^ ^(y-1) == ^(x ^ (y-1)) == -((x ^ (y-1)) + 1)
    mpd_t *y1 = qsub_one(qabs(y));
    mpd_t *z = qadd_one(qxor(x, y1));
    return opa_bf_to_number(qneg(z));
}

OPA_BUILTIN
opa_value *opa_bits_shiftleft(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_bf_to_bf_bits(opa_number_to_bf(a));
    if (x == NULL)
    {
        opa_mpd_del(x);
        return NULL;
    }

    if (opa_value_type(b) != OPA_NUMBER)
    {
        mpd_del(x);
        return NULL;
    }

    long long n;
    if (opa_number_try_int(opa_cast_number(b), &n) || n < 0)
    {
        mpd_del(x);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qshiftn(r, x, n, mpd_max_ctx(), &status);
    mpd_del(x);

    if (status)
    {
        opa_abort("opa_bits_shift");
    }

    return opa_bf_to_number(opa_bf_bits_to_bf(r));
}

OPA_BUILTIN
opa_value *opa_bits_shiftright(opa_value *a, opa_value *b)
{
    mpd_t *x = opa_number_to_bf(a);
    if (x == NULL || !mpd_isinteger(x))
    {
        return NULL;
    }

    if (opa_value_type(b) != OPA_NUMBER)
    {
        mpd_del(x);
        return NULL;
    }

    long long n;
    if (opa_number_try_int(opa_cast_number(b), &n) || n < 0)
    {
        mpd_del(x);
        return NULL;
    }

    if (mpd_sign(x))
    {
        // (-x) >> s == ^(x-1) >> s == ^((x-1) >> s) == -(((x-1) >> s) + 1)
        mpd_t *t = qsub_one(qabs(x));

        t = opa_bf_to_bf_bits(t);
        if (t == NULL)
        {
            // Not an integer.
            return NULL;
        }

        mpd_qshiftr_inplace(t, n);

        mpd_t *z = qneg(qadd_one(opa_bf_bits_to_bf(t)));
        return opa_bf_to_number(z);
    }

    x = opa_bf_to_bf_bits(x);
    if (x == NULL)
    {
        // Not an integer.
        return NULL;
    }

    mpd_qshiftr_inplace(x, n);

    return opa_bf_to_number(opa_bf_bits_to_bf(x));
}
