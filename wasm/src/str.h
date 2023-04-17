#ifndef OPA_STR_H
#define OPA_STR_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

size_t opa_strlen(const char *s);
int opa_strncmp(const char *a, const char *b, int num);
int opa_strcmp(const char *a, const char *b);
int opa_isdigit(char b);
int opa_isspace(char b);
int opa_ishex(char b);
char *opa_itoa(long long i, char *str, int base);
char *opa_reverse(char *str);
int opa_atoi64(const char *str, int len, long long *i);
int opa_atof64(const char *str, int len, double *d);

#ifdef __cplusplus
}
#endif

#endif
