#ifndef OPA_UNICODE_H
#define OPA_UNICODE_H

#include "std.h"

#ifdef __cplusplus
extern "C" {
#endif

int opa_unicode_decode_surrogate(int codepoint1, int codepoint2);
int opa_unicode_decode_unit(const char *in, int i, int len);
int opa_unicode_decode_utf8(const char *in, int i, int len, int *olen);
int opa_unicode_encode_utf8(int codepoint, char *out);
bool opa_unicode_is_space(int codepoint);
int opa_unicode_last_utf8(const char *in, int start, int end);
bool opa_unicode_surrogate(int codepoint);
int opa_unicode_to_lower(int codepoint);
int opa_unicode_to_upper(int codepoint);

#ifdef __cplusplus
}
#endif

#endif
