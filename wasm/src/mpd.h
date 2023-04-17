#ifndef OPA_MPD_H
#define OPA_MPD_H

#include <mpdecimal.h>

typedef struct opa_value opa_value;

void opa_mpd_init(void);
mpd_context_t *mpd_default_ctx(void);
mpd_context_t *mpd_max_ctx(void);
void opa_mpd_del(mpd_t *v);
mpd_t *opa_number_to_bf(opa_value *v);
opa_value *opa_bf_to_number(mpd_t *n);
opa_value *opa_bf_to_number_no_free (mpd_t *n);
mpd_t *qabs(mpd_t *v);
mpd_t *qadd_one(mpd_t *v);
mpd_t *qadd(mpd_t *a, mpd_t *b);
mpd_t *qsub_one(mpd_t *v);
mpd_t *qmul(mpd_t *a, mpd_t *b);
mpd_t *qand(mpd_t *x, mpd_t *y);
mpd_t *qand_not(mpd_t *x, mpd_t *y);
mpd_t *qor(mpd_t *x, mpd_t *y);
mpd_t *qxor(mpd_t *x, mpd_t *y);
mpd_t *qneg(mpd_t *x);

mpd_t *opa_bf_to_bf_bits(mpd_t *v);
mpd_t *opa_bf_bits_to_bf(mpd_t *v);

#endif
