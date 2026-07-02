#include "numbers.h"
#include "mpd.h"
#include "std.h"

OPA_BUILTIN
opa_value *opa_numbers_range(opa_value *v1, opa_value *v2)
{
    mpd_t *i1 = NULL;
    mpd_t *i2 = NULL;
    opa_value *result = NULL;

    i1 = opa_number_to_bf(v1);

    if (i1 == NULL)
    {
        goto cleanup;
    }
    else if (!mpd_isinteger(i1))
    {
        goto cleanup;
    }

    i2 = opa_number_to_bf(v2);

    if (i2 == NULL)
    {
        goto cleanup;
    }
    else if (!mpd_isinteger(i2))
    {
        goto cleanup;
    }

    uint32_t status = 0;
    int cmp = mpd_qcmp(i1, i2, &status);

    if (status)
    {
        opa_abort("opa_numbers_range: comparison");
    }

    result = opa_array();
    opa_array_t *arr = opa_cast_array(result);

    if (cmp <= 0)
    {
        mpd_t *curr = i1;
        i1 = NULL;

        while (cmp <= 0)
        {
            opa_value *add = opa_bf_to_number_no_free(curr);

            if (add == NULL)
            {
                opa_abort("opa_numbers_range: conversion");
            }

            opa_array_append(arr, add);
            curr = qadd_one(curr);
            cmp = mpd_qcmp(curr, i2, &status);

            if (status)
            {
                opa_abort("opa_numbers_range: comparison");
            }
        }

        opa_mpd_del(curr);
    }
    else
    {
        mpd_t *curr = i1;
        i1 = NULL;

        while (cmp >= 0)
        {
            opa_value *add = opa_bf_to_number_no_free(curr);

            if (add == NULL)
            {
                opa_abort("opa_numbers_range: conversion");
            }

            opa_array_append(arr, add);
            curr = qsub_one(curr);
            cmp = mpd_qcmp(curr, i2, &status);

            if (status)
            {
                opa_abort("opa_numbers_range: comparison");
            }
        }

        opa_mpd_del(curr);
    }

cleanup:
    opa_mpd_del(i1);
    opa_mpd_del(i2);

    return result;
}
