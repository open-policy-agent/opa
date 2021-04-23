#include <mpdecimal.h>

#include "str.h"
#include "value.h"

static int initialized;
static mpd_context_t default_ctx;
static mpd_context_t max_ctx;

OPA_INTERNAL
void opa_mpd_init(void)
{
    if (!initialized)
    {
        mpd_defaultcontext(&default_ctx);
        default_ctx.traps = 0;

        mpd_maxcontext(&max_ctx);
        max_ctx.traps = 0;
        max_ctx.round = MPD_ROUND_HALF_UP; // .5 always rounded up

        initialized = 1;
    }
}

mpd_context_t *mpd_default_ctx(void)
{
    return &default_ctx;
}

mpd_context_t *mpd_max_ctx(void)
{
    return &max_ctx;
}

static mpd_uint_t const_one[MPD_MINALLOC_MAX] = {1};
static mpd_t one = {MPD_STATIC|MPD_STATIC_DATA|MPD_SHARED_DATA, 0, 1, 1, MPD_MINALLOC_MAX, const_one};
static mpd_t minus_one = {MPD_STATIC|MPD_STATIC_DATA|MPD_SHARED_DATA|MPD_NEG, 0, 1, 1, MPD_MINALLOC_MAX, const_one};

mpd_t *mpd_one(void)
{
    return &one;
}

mpd_t *mpd_minus_one(void)
{
    return &minus_one;
}

void opa_mpd_del(mpd_t *v)
{
    if (v != NULL)
    {
        mpd_del(v);
    }
}

mpd_t *opa_number_to_bf(opa_value *v)
{
    if (opa_value_type(v) != OPA_NUMBER)
    {
        return NULL;
    }
    opa_number_t *n = opa_cast_number(v);
    switch (n->repr)
    {
    case OPA_NUMBER_REPR_INT: {
        uint32_t status = 0;
        mpd_t *r = mpd_qnew();
        mpd_set_static(r);
        mpd_set_static_data(r);
        mpd_qset_i64(r, n->v.i, mpd_default_ctx(), &status);
        if (status != 0)
        {
            opa_abort("opa_number_to_bf: invalid number");
        }
        n->repr = OPA_NUMBER_REPR_MPD;
        n->v.mpd.d = r;
        n->v.mpd.free = 1;
        return r;
    }

    case OPA_NUMBER_REPR_MPD:
        return n->v.mpd.d; // has static/static_data set, won't be deleted by mpd_del

    default:
        opa_abort("opa_number_to_bf: illegal repr");
        return NULL;
    }
}

int opa_mpd_try_int(mpd_t *d, long long *i)
{
    if (!mpd_isinteger(d))
    {
        return -1;
    }

    uint32_t status = 0;
    int64_t w = mpd_qget_i64(d, &status);
    if (status != 0)
    {
        return -1;
    }
    *i = w;
    return 0;
}

/* converts an big number to a bigint with base of 10 and digits of 0 and 1. */
mpd_t *opa_bf_to_bf_bits(mpd_t *v)
{
    if (v == NULL)
    {
        return NULL;
    }

    mpd_t *i = mpd_qnew();
    uint32_t status = 0;

    mpd_qround_to_intx(i, v, mpd_max_ctx(), &status);
    if (status)
    {
        mpd_del(i);
        return NULL;
    }

    int c = mpd_qcmp(i, v, &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    mpd_del(v);

    if (c != 0)
    {
        // Not an integer value.
        mpd_del(i);
        return NULL;
    }

    uint8_t sign = MPD_POS;
    if (mpd_sign(i))
    {
        sign = MPD_NEG;

        v = mpd_qnew();
        mpd_qabs(v, i, mpd_max_ctx(), &status);
        if (status)
        {
            opa_abort("opa_bits: bits conversion");
        }

        mpd_del(i);
        i = v;
    }

    size_t rlen = mpd_sizeinbase(i, 2);
    uint16_t *rdata = malloc(rlen * sizeof(uint16_t));
    size_t digits = mpd_qexport_u16(&rdata, rlen, 2, i, &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    mpd_del(i);

    mpd_t *bits = mpd_qnew();
    mpd_qimport_u16(bits, rdata, digits, sign, 10, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    free(rdata);

    return bits;
}

mpd_t *opa_bf_bits_to_bf(mpd_t *v)
{
    if (v == NULL)
    {
        return NULL;
    }

    uint8_t sign = MPD_POS;
    uint32_t status = 0;

    if (mpd_sign(v))
    {
        mpd_t *abs = mpd_qnew();
        mpd_qabs(abs, v, mpd_max_ctx(), &status);
        if (status)
        {
            opa_abort("opa_bits: bits conversion");
        }

        mpd_del(v);
        v = abs;

        sign = MPD_NEG;
    }

    size_t rlen = mpd_sizeinbase(v, 10);
    uint16_t *rdata = malloc(rlen * sizeof(uint16_t));
    size_t digits = mpd_qexport_u16(&rdata, rlen, 10, v, &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    mpd_del(v);

    mpd_t *i = mpd_qnew();
    mpd_qimport_u16(i, rdata, digits, sign, 2, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    free(rdata);

    return i;
}

mpd_t *qabs(mpd_t *v)
{
    if (v == NULL)
    {
        return NULL;
    }

    mpd_t *a = mpd_qnew();
    uint32_t status = 0;

    mpd_qabs(a, v, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: abs conversion");
    }

    mpd_del(v);
    return a;
}

mpd_t *qadd_one(mpd_t *v)
{
    if (v == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qadd(r, v, mpd_one(), mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: add one");
    }

    mpd_del(v);
    return r;
}

mpd_t *qadd(mpd_t *a, mpd_t *b)
{
    if (a == NULL || b == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qadd(r, a, b, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: add");
    }

    mpd_del(a);
    mpd_del(b);
    return r;
}

mpd_t *qsub_one(mpd_t *v)
{
    if (v == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qsub(r, v, mpd_one(), mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: minus one");
    }

    mpd_del(v);
    return r;
}

mpd_t *qmul(mpd_t *a, mpd_t *b)
{
    if (a == NULL || b == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qmul(r, a, b, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: mul");
    }

    mpd_del(a);
    mpd_del(b);
    return r;
}

mpd_t *qand(mpd_t *x, mpd_t *y)
{
    x = opa_bf_to_bf_bits(x);
    y = opa_bf_to_bf_bits(y);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qand(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status)
    {
        opa_abort("opa_bits_and");
    }

    return opa_bf_bits_to_bf(r);
}

mpd_t *qand_not(mpd_t *x, mpd_t *y)
{
    x = opa_bf_to_bf_bits(x);
    y = opa_bf_to_bf_bits(y);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    uint32_t status = 0;

    // ^y = y ^ 1111...
    size_t rlenx = mpd_sizeinbase(x, 10);
    size_t rleny = mpd_sizeinbase(y, 10);
    size_t rlen = rlenx < rleny ? rleny : rlenx;
    uint16_t *rdata = malloc(rlen * sizeof(uint16_t));
    size_t digits = mpd_qexport_u16(&rdata, rlen, 10, y, &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    for (int i = 0; i < rlen; i++)
    {
        rdata[i] = 1;
    }

    mpd_t *mask = mpd_qnew();
    mpd_qimport_u16(mask, rdata, rlen, MPD_POS, 10, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits: bits conversion");
    }

    free(rdata);

    mpd_t *ny = mpd_qnew();
    mpd_qxor(ny, y, mask, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_bits_negate");
    }

    mpd_del(y);
    mpd_del(mask);

    mpd_t *r = mpd_qnew();
    mpd_qand(r, x, ny, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(ny);

    if (status)
    {
        opa_abort("opa_bits_and_not");
    }

    return opa_bf_bits_to_bf(r);
}

mpd_t *qor(mpd_t *x, mpd_t *y)
{
    x = opa_bf_to_bf_bits(x);
    y = opa_bf_to_bf_bits(y);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qor(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status)
    {
        opa_abort("opa_bits_or");
    }

    return opa_bf_bits_to_bf(r);
}

mpd_t *qxor(mpd_t *x, mpd_t *y)
{
    x = opa_bf_to_bf_bits(x);
    y = opa_bf_to_bf_bits(y);
    if (x == NULL || y == NULL)
    {
        opa_mpd_del(x);
        opa_mpd_del(y);
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qxor(r, x, y, mpd_max_ctx(), &status);
    mpd_del(x);
    mpd_del(y);

    if (status)
    {
        opa_abort("opa_bits_xor");
    }

    return opa_bf_bits_to_bf(r);
}

mpd_t *qneg(mpd_t *x)
{
    x = opa_bf_to_bf_bits(x);
    if (x == NULL)
    {
        return NULL;
    }

    mpd_t *r = mpd_qnew();
    uint32_t status = 0;

    mpd_qminus(r, x, mpd_max_ctx(), &status);
    mpd_del(x);

    if (status)
    {
        opa_abort("opa_bits_neg");
    }

    return opa_bf_bits_to_bf(r);
}
