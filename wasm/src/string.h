#ifndef OPA_STRING_H
#define OPA_STRING_H

#include "std.h"

size_t opa_strlen(const char *s);
double opa_strntod(const char *s, int len);
int opa_strncmp(const char *a, const char *b, int num);
int opa_strcmp(const char *a, const char *b);
int opa_isdigit(char b);
int opa_isspace(char b);
int opa_ishex(char b);

#endif