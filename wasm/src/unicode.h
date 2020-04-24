#ifndef OPA_UNICODE_H
#define OPA_UNICODE_H

int opa_unicode_decode_surrogate(int codepoint1, int codepoint2);
int opa_unicode_decode_unit(const char *in, int i, int len);
int opa_unicode_decode_utf8(const char *in, int i, int len, int *olen);
int opa_unicode_encode_utf8(int rune, char *out);
int opa_unicode_surrogate(int codepoint);

#endif
