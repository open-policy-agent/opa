#include "numbers.h"
#include "mpd.h"
#include "std.h"

OPA_BUILTIN
opa_value *opa_numbers_range(opa_value *v1, opa_value *v2)
{
    mpd_t *i1 = opa_number_to_bf(v1);
    mpd_t *i2 = opa_number_to_bf(v2);

    if (i1 == NULL || i2 == NULL ||
        !mpd_isinteger(i1) || !mpd_isinteger(i2))
    {
        return NULL;
    }

    uint32_t status = 0;
    int cmp = mpd_qcmp(i1, i2, &status);
    if (status)
    {
        opa_abort("opa_numbers_range: comparison");
    }

    opa_array_t *arr = opa_cast_array(opa_array());
    
    // step: 1 or -1
    mpd_t *step = (cmp <= 0) ? mpd_one() : mpd_minus_one();

    // count: abs(a-b)
    mpd_t *diff = mpd_qnew();
    status = 0;
    mpd_qsub(diff, i1, i2, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_numbers_range: sub");
    }

    status = 0;
    mpd_qabs(diff, diff, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("opa_numbers_range: abs");
    }

    long long n;
    if (opa_mpd_try_int(diff, &n))
    {
        opa_abort("opa_numbers_range: int");
    }
    mpd_del(diff);

    mpd_t *curr = mpd_qncopy(i1);
    while (n-- >= 0)
    {
        mpd_t *cpy = mpd_qncopy(curr);
        opa_array_append(arr, opa_number_mpd_allocated(cpy));
        status = 0;
        mpd_qadd(curr, curr, step, mpd_max_ctx(), &status);
        if (status)
        {
            opa_abort("opa_numbers_range: add");
        }
    }

    mpd_del(curr);
    return &arr->hdr;
}
