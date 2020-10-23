#ifndef OPA_JSON_H
#define OPA_JSON_H

#include "value.h"

typedef struct
{
    const char *input;
    size_t len;
    const char *buf;
    const char *buf_end;
    const char *curr;
    int set_literals_enabled;
} opa_json_lex;

#define OPA_JSON_TOKEN_ERROR 0
#define OPA_JSON_TOKEN_EOF 1
#define OPA_JSON_TOKEN_NULL 2
#define OPA_JSON_TOKEN_TRUE 3
#define OPA_JSON_TOKEN_FALSE 4
#define OPA_JSON_TOKEN_NUMBER 5
#define OPA_JSON_TOKEN_STRING 6
#define OPA_JSON_TOKEN_STRING_ESCAPED 7
#define OPA_JSON_TOKEN_OBJECT_START 8
#define OPA_JSON_TOKEN_OBJECT_END 9
#define OPA_JSON_TOKEN_ARRAY_START 10
#define OPA_JSON_TOKEN_ARRAY_END 11
#define OPA_JSON_TOKEN_COMMA 12
#define OPA_JSON_TOKEN_COLON 13
#define OPA_JSON_TOKEN_EMPTY_SET 14

void opa_json_lex_init(const char *input, size_t len, opa_json_lex *ctx);
int opa_json_lex_read(opa_json_lex *ctx);

opa_value *opa_json_parse(const char *input, size_t len);
opa_value *opa_value_parse(const char *input, size_t len);
const char *opa_json_dump(opa_value *v);
const char *opa_value_dump(opa_value *v);

size_t opa_json_max_string_len(const char *input, size_t len);

#endif
