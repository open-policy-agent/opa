#include <stdio.h>

#include "str.h"
#include "value.h"
#include "json.h"
#include "malloc.h"
#include "unicode.h"

static opa_value *opa_json_parse_token(opa_json_lex *ctx, int token);

int opa_json_lex_offset(opa_json_lex *ctx)
{
    return ctx->curr - ctx->input;
}

int opa_json_lex_remaining(opa_json_lex *ctx)
{
    return ctx->len - opa_json_lex_offset(ctx);
}

int opa_json_lex_eof(opa_json_lex *ctx)
{
    return (ctx->curr - ctx->input) >= ctx->len;
}

int opa_json_lex_read_atom(opa_json_lex *ctx, const char *str, int n, int token)
{
    if (opa_json_lex_remaining(ctx) >= n)
    {
        if (opa_strncmp(str, ctx->curr, n) == 0)
        {
            ctx->curr += n;
            return token;
        }
    }
    return OPA_JSON_TOKEN_ERROR;
}

void opa_json_lex_read_digits(opa_json_lex *ctx)
{
    while (!opa_json_lex_eof(ctx) && opa_isdigit(*ctx->curr))
    {
        ctx->curr++;
    }
}

int opa_json_lex_read_unicode(opa_json_lex *ctx)
{
    if (opa_json_lex_remaining(ctx) >= 4)
    {
        for (int i = 0; i < 4; i++)
        {
            if (!opa_ishex(ctx->curr[i]))
            {
                return -1;
            }
        }
        ctx->curr += 4;
        return 0;
    }

    return 1;
}

int opa_json_lex_read_number(opa_json_lex *ctx)
{
    ctx->buf = ctx->curr;

    // Handle sign component.
    if (*ctx->curr == '-')
    {
        ctx->curr++;

        if (opa_json_lex_eof(ctx))
        {
            goto err;
        }
    }

    // Handle integer component.
    if (*ctx->curr == '0')
    {
        ctx->curr++;
    }
    else if (opa_isdigit(*ctx->curr))
    {
        opa_json_lex_read_digits(ctx);
    }
    else
    {
        goto err;
    }

    if (opa_json_lex_eof(ctx))
    {
        goto out;
    }

    // Handle fraction component.
    if (*ctx->curr == '.')
    {
        ctx->curr++;
        opa_json_lex_read_digits(ctx);

        if (opa_json_lex_eof(ctx))
        {
            goto out;
        }
    }

    // Handle exponent component.
    if (*ctx->curr == 'e' || *ctx->curr == 'E')
    {
        ctx->curr++;

        if (opa_json_lex_eof(ctx))
        {
            goto err;
        }

        if (*ctx->curr == '+' || *ctx->curr == '-')
        {
            ctx->curr++;

            if (opa_json_lex_eof(ctx))
            {
                goto err;
            }
        }

        opa_json_lex_read_digits(ctx);
    }

out:
    ctx->buf_end = ctx->curr;
    return OPA_JSON_TOKEN_NUMBER;

err:
    return OPA_JSON_TOKEN_ERROR;
}

int opa_json_lex_read_string(opa_json_lex *ctx)
{
    if (*ctx->curr != '"')
    {
        goto err;
    }

    ctx->buf = ++ctx->curr;
    int escaped = 0;

    while (1)
    {
        if (opa_json_lex_eof(ctx))
        {
            goto err;
        }

        unsigned char b = *ctx->curr;

        switch (b)
        {
        case '\\':
            escaped = 1;
            ctx->curr++;

            if (opa_json_lex_eof(ctx))
            {
                goto err;
            }

            b = *ctx->curr;

            switch (b)
            {
            case '"':
            case '\\':
            case '/':
            case 'b':
            case 'f':
            case 'n':
            case 'r':
            case 't':
                ctx->curr++;
                break;
            case 'u':
                ctx->curr++;
                if (opa_json_lex_read_unicode(ctx) != 0)
                {
                    goto err;
                }
                break;
            default:
                goto err;
            }

            break;
        case '"':
            goto out;

        default:
            if (b < ' ') {
                goto err;
            }

            if (b > '~')
            {
                // Revert to slow path to validate UTF-8 encoding.
                escaped = 1;
            }
            ctx->curr++;
            break;
        }
    }
out:
    ctx->buf_end = ctx->curr++;

    if (escaped)
    {
        return OPA_JSON_TOKEN_STRING_ESCAPED;
    }

    return OPA_JSON_TOKEN_STRING;

err:
    return OPA_JSON_TOKEN_ERROR;
}

int opa_json_lex_read_empty_set(opa_json_lex *ctx)
{
    if (!ctx->set_literals_enabled)
    {
        return OPA_JSON_TOKEN_ERROR;
    }

    int token = opa_json_lex_read_atom(ctx, "set(", 4, OPA_JSON_TOKEN_EMPTY_SET);

    if (token != OPA_JSON_TOKEN_EMPTY_SET)
    {
        return OPA_JSON_TOKEN_ERROR;
    }

    while (opa_isspace(*ctx->curr))
    {
        ctx->curr++;
    }

    if (*ctx->curr != ')')
    {
        return OPA_JSON_TOKEN_ERROR;
    }

    ctx->curr++;
    return token;
}

int opa_json_lex_read(opa_json_lex *ctx)
{
    while (!opa_json_lex_eof(ctx))
    {
        char b = *ctx->curr;
        switch (b)
        {
        case 'n':
            return opa_json_lex_read_atom(ctx, "null", 4, OPA_JSON_TOKEN_NULL);
        case 't':
            return opa_json_lex_read_atom(ctx, "true", 4, OPA_JSON_TOKEN_TRUE);
        case 'f':
            return opa_json_lex_read_atom(ctx, "false", 5, OPA_JSON_TOKEN_FALSE);
        case 's':
            return opa_json_lex_read_empty_set(ctx);
        case '"':
            return opa_json_lex_read_string(ctx);
        case '{':
            ctx->curr++;
            return OPA_JSON_TOKEN_OBJECT_START;
        case '}':
            ctx->curr++;
            return OPA_JSON_TOKEN_OBJECT_END;
        case '[':
            ctx->curr++;
            return OPA_JSON_TOKEN_ARRAY_START;
        case ']':
            ctx->curr++;
            return OPA_JSON_TOKEN_ARRAY_END;
        case ',':
            ctx->curr++;
            return OPA_JSON_TOKEN_COMMA;
        case ':':
            ctx->curr++;
            return OPA_JSON_TOKEN_COLON;
        default:
            if (opa_isdigit(b) || b == '-')
            {
                return opa_json_lex_read_number(ctx);
            }
            else if (opa_isspace(b))
            {
                ctx->curr++;
                continue;
            }
            return OPA_JSON_TOKEN_ERROR;
        }
    }

    return OPA_JSON_TOKEN_EOF;
}

void opa_json_lex_init(const char *input, size_t len, opa_json_lex *ctx)
{
    ctx->input = input;
    ctx->len = len;
    ctx->curr = input;
    ctx->buf = NULL;
    ctx->buf_end = NULL;
    ctx->set_literals_enabled = 0;
}

size_t opa_json_max_string_len(const char *buf, size_t len)
{
    // The lexer will catch invalid escaping, e.g., if the last char in the
    // buffer is reverse solidus this will be caught ahead-of-time.
    int skip = 0;

    for (int i = 0; i < len; i++)
    {
        if (buf[i] == '\\')
        {
            int codepoint;

            codepoint = opa_unicode_decode_unit(buf, i, len);
            if (codepoint == -1) {
                // If not a codepoint \uXXXX, must be a single
                // character escaping.
                skip++;
                i++;
                continue;
            }

            i += 5;

            // Assume each UTF-16 encoded character to take full 4
            // bytes when encoded as UTF-8.  However, if encoded as a
            // surrogate pair, it's split to two 2 bytes.
            if (!opa_unicode_surrogate(codepoint)) {
                skip += 2;
                continue;
            }

            skip += 4;
        }
    }

    return len - skip;
}

opa_value *opa_json_parse_string(int token, const char *buf, int len)
{
    if (token == OPA_JSON_TOKEN_STRING)
    {
        char *cpy = (char *)opa_malloc(len);

        for (int i = 0; i < len; i++)
        {
            cpy[i] = buf[i];
        }

        return opa_string_allocated(cpy, len);
    }

    int max_len = opa_json_max_string_len(buf, len);
    char *cpy = (char *)opa_malloc(max_len);
    char *out = cpy;

    for (int i = 0; i < len;)
    {
        unsigned char c = buf[i];

        if (c != '\\')
        {
            if (c < ' ' || c == '"')
            {
                opa_abort("illegal unescaped character");
            }

            if (c < 0x80)
            {
                *out++ = c;
                i++;
            } else {
                int n;
                int cp = opa_unicode_decode_utf8(buf, i, len, &n);
                if (cp == -1)
                {
                    opa_abort("illegal utf-8");
                }

                i += n;

                n = opa_unicode_encode_utf8(cp, out);
                out += n;
            }

            continue;
        }

        char next = buf[i+1];

        switch (next)
        {
            case '"':
            case '\\':
            case '/':
                *out++ = next;
                i += 2;
                break;
            case 'b':
                *out++ = '\b';
                i += 2;
                break;
            case 'f':
                *out++ = '\f';
                i += 2;
                break;
            case 'n':
                *out++ = '\n';
                i += 2;
                break;
            case 'r':
                *out++ = '\r';
                i += 2;
                break;
            case 't':
                *out++ = '\t';
                i += 2;
                break;
            case 'u':
                {
                    // JSON encodes unicode characters as UTF-16 that
                    // have either a single or two code units.  If two
                    // code units, the character is represented as a
                    // pair of UTF-16 surrogates.  Surrogates don't
                    // overlap with characters that can be encoded as
                    // a single value.
                    int u = opa_unicode_decode_unit(buf, i, len);
                    if (u == -1) {
                        opa_abort("illegal string escape character");
                    }

                    i += 6;

                    if (opa_unicode_surrogate(u)) {
                        int v = opa_unicode_decode_unit(buf, i, len);
                        if (v == -1) {
                            opa_abort("illegal string escape character");
                        }

                        u = opa_unicode_decode_surrogate(u, v);
                        i += 6;
                    }

                    out += opa_unicode_encode_utf8(u, out);
                    break;
                }
            default:
                // this is unreachable.
                opa_abort("illegal string escape character");
        }
    }

    return opa_string_allocated(cpy, out-cpy);
}

opa_value *opa_json_parse_number(const char *buf, int len)
{
    char *cpy = (char *)opa_malloc(len);

    for (int i = 0; i < len; i++)
    {
        cpy[i] = buf[i];
    }

    return opa_number_ref_allocated(cpy, len);
}

opa_value *opa_json_parse_array(opa_json_lex *ctx)
{
    opa_value *ret = opa_array();
    opa_array_t *arr = opa_cast_array(ret);
    int sep = 0;

    while (1)
    {
        int token = opa_json_lex_read(ctx);

        switch (token)
        {
        case OPA_JSON_TOKEN_ARRAY_END:
            return ret;
        case OPA_JSON_TOKEN_COMMA:
            if (sep)
            {
                sep = 0;
                continue;
            }
        }

        opa_value *elem = opa_json_parse_token(ctx, token);

        if (elem == NULL)
        {
            return NULL;
        }

        opa_array_append(arr, elem);
        sep = 1;
    }
}

opa_value *opa_json_parse_set(opa_json_lex *ctx, opa_value *elem, int token)
{
    if (!ctx->set_literals_enabled)
    {
        return NULL;
    }

    opa_set_t *set = opa_cast_set(opa_set());
    opa_set_add(set, elem);

    if (token == OPA_JSON_TOKEN_OBJECT_END)
    {
        return &set->hdr;
    }

    token = opa_json_lex_read(ctx);

    while (1)
    {
        elem = opa_json_parse_token(ctx, token);

        if (elem == NULL)
        {
            return NULL;
        }

        opa_set_add(set, elem);
        token = opa_json_lex_read(ctx);

        switch (token)
        {
        case OPA_JSON_TOKEN_COMMA:
            token = opa_json_lex_read(ctx);
            break;
        case OPA_JSON_TOKEN_OBJECT_END:
            return &set->hdr;
        default:
            return NULL;
        }
    }

    return NULL;
}

opa_value *opa_json_parse_object(opa_json_lex *ctx, opa_value *key)
{
    int token = opa_json_lex_read(ctx);
    opa_value *val = opa_json_parse_token(ctx, token);

    if (val == NULL)
    {
        return NULL;
    }

    opa_object_t *obj = opa_cast_object(opa_object());
    opa_object_insert(obj, key, val);
    token = opa_json_lex_read(ctx);

    switch (token)
    {
    case OPA_JSON_TOKEN_OBJECT_END:
        return &obj->hdr;
    case OPA_JSON_TOKEN_COMMA:
        break;
    default:
        return NULL;
    }

    token = opa_json_lex_read(ctx);

    while (1)
    {
        key = opa_json_parse_token(ctx, token);

        if (key == NULL)
        {
            return NULL;
        }

        token = opa_json_lex_read(ctx);

        if (token != OPA_JSON_TOKEN_COLON)
        {
            return NULL;
        }

        token = opa_json_lex_read(ctx);
        val = opa_json_parse_token(ctx, token);

        if (val == NULL)
        {
            return NULL;
        }

        opa_object_insert(obj, key, val);
        token = opa_json_lex_read(ctx);

        switch (token)
        {
        case OPA_JSON_TOKEN_OBJECT_END:
            return &obj->hdr;
        case OPA_JSON_TOKEN_COMMA:
        {
            token = opa_json_lex_read(ctx);
            break;
        }
        default:
            return NULL;
        }
    }

    return NULL;
}

opa_value *opa_json_parse_object_or_set(opa_json_lex *ctx)
{
    int token = opa_json_lex_read(ctx);

    if (token == OPA_JSON_TOKEN_OBJECT_END)
    {
        return opa_object();
    }

    opa_value *head = opa_json_parse_token(ctx, token);

    if (head == NULL)
    {
        return NULL;
    }

    token = opa_json_lex_read(ctx);

    switch (token)
    {
    case OPA_JSON_TOKEN_OBJECT_END:
    case OPA_JSON_TOKEN_COMMA:
        return opa_json_parse_set(ctx, head, token);
    case OPA_JSON_TOKEN_COLON:
        return opa_json_parse_object(ctx, head);
    }

    return NULL;
}

opa_value *opa_json_parse_token(opa_json_lex *ctx, int token)
{
    switch (token)
    {
    case OPA_JSON_TOKEN_NULL:
        return opa_null();
    case OPA_JSON_TOKEN_TRUE:
        return opa_boolean(TRUE);
    case OPA_JSON_TOKEN_FALSE:
        return opa_boolean(FALSE);
    case OPA_JSON_TOKEN_NUMBER:
        return opa_json_parse_number(ctx->buf, ctx->buf_end - ctx->buf);
    case OPA_JSON_TOKEN_STRING:
    case OPA_JSON_TOKEN_STRING_ESCAPED:
        return opa_json_parse_string(token, ctx->buf, ctx->buf_end - ctx->buf);
    case OPA_JSON_TOKEN_ARRAY_START:
        return opa_json_parse_array(ctx);
    case OPA_JSON_TOKEN_OBJECT_START:
        return opa_json_parse_object_or_set(ctx);
    case OPA_JSON_TOKEN_EMPTY_SET:
        return opa_set();
    default:
        return NULL;
    }
}

opa_value *opa_json_parse(const char *input, size_t len)
{
    opa_json_lex ctx;
    opa_json_lex_init(input, len, &ctx);
    int token = opa_json_lex_read(&ctx);
    return opa_json_parse_token(&ctx, token);
}

opa_value *opa_value_parse(const char *input, size_t len)
{
    opa_json_lex ctx;
    opa_json_lex_init(input, len, &ctx);
    ctx.set_literals_enabled = 1;
    int token = opa_json_lex_read(&ctx);
    return opa_json_parse_token(&ctx, token);
}

typedef struct {
    char *buf;
    char *next;
    size_t len;
    int set_literals_enabled;
    int non_string_object_keys_enabled;
} opa_json_writer;

void opa_json_writer_init(opa_json_writer *w)
{
    w->buf = NULL;
    w->next = NULL;
    w->len = 0;
    w->set_literals_enabled = 0;
    w->non_string_object_keys_enabled = 0;
}

size_t opa_json_writer_offset(opa_json_writer *w)
{
    return w->next - w->buf;
}

size_t opa_json_writer_space(opa_json_writer *w)
{
    return w->len - opa_json_writer_offset(w);
}

int opa_json_writer_grow(opa_json_writer *w, size_t newlen, size_t copy)
{
    char *newbuf = (char *)opa_malloc(newlen);

    if (newbuf == NULL)
    {
        return -1;
    }

    for (size_t i = 0; i < copy; i++)
    {
        newbuf[i] = w->buf[i];
    }

    size_t offset = opa_json_writer_offset(w);

    w->buf = newbuf;
    w->next = newbuf + offset;
    w->len = newlen;

    return 0;
}

int opa_json_writer_emit_chars(opa_json_writer *w, const char *bs, size_t nb)
{
    size_t offset = opa_json_writer_offset(w);

    if (offset + nb > w->len)
    {
        int rc = opa_json_writer_grow(w, (offset + nb) * 2, w->len);

        if (rc != 0)
        {
            return rc;
        }
    }

    for(int i = 0; i < nb; i++)
    {
        w->next[i] = bs[i];
    }

    w->next += nb;

    return 0;
}

int opa_json_writer_emit_char(opa_json_writer *w, char b)
{
    char bs[] = {b};

    return opa_json_writer_emit_chars(w, bs, 1);
}

int opa_json_writer_emit_null(opa_json_writer *w)
{
    char bs[] = "null";

    return opa_json_writer_emit_chars(w, bs, sizeof(bs)-1);
}

int opa_json_writer_emit_boolean(opa_json_writer *w, opa_boolean_t *b)
{
    if (b->v == 0)
    {
        char bs[] = "false";

        return opa_json_writer_emit_chars(w, bs, sizeof(bs)-1);
    }

    char bs[] = "true";

    return opa_json_writer_emit_chars(w, bs, sizeof(bs)-1);
}

int opa_json_writer_emit_float(opa_json_writer *w, double f)
{
    char str[32];
    snprintf(str, sizeof(str), "%g", f);
    return opa_json_writer_emit_chars(w, str, opa_strlen(str));
}

int opa_json_writer_emit_integer(opa_json_writer *w, long long i)
{
    char str[sizeof(i)*8+1]; // once base=2 is supported we need 8 bits per byte.
    opa_itoa(i, str, 10);
    return opa_json_writer_emit_chars(w, str, opa_strlen(str));
}

int opa_json_writer_emit_number(opa_json_writer *w, opa_number_t *n)
{
    switch (n->repr)
    {
    case OPA_NUMBER_REPR_FLOAT:
        return opa_json_writer_emit_float(w, n->v.f);
    case OPA_NUMBER_REPR_INT:
        return opa_json_writer_emit_integer(w, n->v.i);
    case OPA_NUMBER_REPR_REF:
        return opa_json_writer_emit_chars(w, n->v.ref.s, n->v.ref.len);
    default:
        opa_abort("opa_json_writer_emit_number: illegal repr");
        return -1;
    }
}

int opa_json_writer_emit_string(opa_json_writer *w, opa_string_t *s)
{
    int rc = opa_json_writer_emit_char(w, '"');

    if (rc != 0)
    {
        return rc;
    }

    for (size_t i = 0; i < s->len; i++)
    {
        // Encode any character below 32 (space) with \u00XX, unless
        // \n, \r or \t.  including and above character 32, escape if
        // \ or ". Anything else is expected to be valid UTF-8.

        unsigned char c = s->v[i];
        if (c >= ' ' && c != '\\' && c != '"')
        {
            rc = opa_json_writer_emit_char(w, c);
            if (rc != 0)
            {
                return rc;
            }

            continue;
        }

        rc = opa_json_writer_emit_char(w, '\\');
        if (rc != 0)
        {
            return rc;
        }

        if (c == '\\' || c == '"') {
            rc = opa_json_writer_emit_char(w, c);
        } else if (c == '\n') {
            rc = opa_json_writer_emit_char(w, 'n');
        } else if (c == '\r') {
            rc = opa_json_writer_emit_char(w, 'r');
        } else if (c == '\t') {
            rc = opa_json_writer_emit_char(w, 't');
        } else {
            rc = opa_json_writer_emit_chars(w, "u00", 3);
            if (rc != 0)
            {
                return rc;
            }

            char buf[3];
            snprintf(buf, 3, "%02x", c);

            rc = opa_json_writer_emit_chars(w, buf, 2);
            if (rc != 0)
            {
                return rc;
            }
        }

        if (rc != 0)
        {
            return rc;
        }
    }

    rc = opa_json_writer_emit_char(w, '"');

    if (rc != 0)
    {
        return rc;
    }

    return 0;
}

int opa_json_writer_emit_value(opa_json_writer *, opa_value *);

int opa_json_writer_emit_array_element(opa_json_writer *w, opa_value *coll, opa_value *k)
{
    return opa_json_writer_emit_value(w, opa_value_get(coll, k));
}

int opa_json_writer_emit_set_element(opa_json_writer *w, opa_value *coll, opa_value *k)
{
    return opa_json_writer_emit_value(w, k);
}

int opa_json_writer_emit_object_element(opa_json_writer *w, opa_value *coll, opa_value *k)
{
    if (w->non_string_object_keys_enabled || opa_value_type(k) == OPA_STRING)
    {
        int rc = opa_json_writer_emit_value(w, k);

        if (rc != 0)
        {
            return rc;
        }
    }
    else
    {
        char *buf = opa_json_dump(k);

        if (buf == NULL)
        {
            return -3;
        }

        opa_value *serialized = opa_string_terminated(buf);
        int rc = opa_json_writer_emit_value(w, serialized);
        opa_value_free(serialized);
        opa_free(buf);

        if (rc != 0)
        {
            return rc;
        }
    }

    int rc = opa_json_writer_emit_char(w, ':');

    if (rc != 0)
    {
        return rc;
    }

    return opa_json_writer_emit_value(w, opa_value_get(coll, k));
}

int opa_json_writer_emit_collection(opa_json_writer *w, opa_value *v, char open, char close, int (*emitfunc)(opa_json_writer *, opa_value *, opa_value *))
{
    int rc = opa_json_writer_emit_char(w, open);

    if (rc != 0)
    {
        return rc;
    }

    opa_value *prev = NULL;
    opa_value *curr = NULL;

    while ((curr = opa_value_iter(v, prev)) != NULL)
    {
        if (prev != NULL)
        {
            rc = opa_json_writer_emit_char(w, ',');

            if (rc != 0)
            {
                return rc;
            }
        }

        rc = emitfunc(w, v, curr);

        if (rc != 0)
        {
            return rc;
        }

        prev = curr;
    }

    return opa_json_writer_emit_char(w, close);
}

int opa_json_writer_emit_set_literal(opa_json_writer *w, opa_set_t *set)
{
    if (opa_value_length(&set->hdr) == 0)
    {
        const char empty_set[] = "set()";

        return opa_json_writer_emit_chars(w, empty_set, sizeof(empty_set)-1);
    }

    return opa_json_writer_emit_collection(w, &set->hdr, '{', '}', opa_json_writer_emit_set_element);
}

int opa_json_writer_emit_value(opa_json_writer *w, opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_NULL:
        return opa_json_writer_emit_null(w);
    case OPA_BOOLEAN:
        return opa_json_writer_emit_boolean(w, opa_cast_boolean(v));
    case OPA_STRING:
        return opa_json_writer_emit_string(w, opa_cast_string(v));
    case OPA_NUMBER:
        return opa_json_writer_emit_number(w, opa_cast_number(v));
    case OPA_ARRAY:
        return opa_json_writer_emit_collection(w, v, '[', ']', opa_json_writer_emit_array_element);
    case OPA_SET:
    {
        if (!w->set_literals_enabled)
        {
            return opa_json_writer_emit_collection(w, v, '[', ']', opa_json_writer_emit_set_element);
        }
        return opa_json_writer_emit_set_literal(w, opa_cast_set(v));
    }
    case OPA_OBJECT:
        return opa_json_writer_emit_collection(w, v, '{', '}', opa_json_writer_emit_object_element);
    }

    return -2;
}

char *opa_json_writer_write(opa_json_writer *w, opa_value *v)
{
    if (opa_json_writer_grow(w, 1024, 0) != 0)
    {
        goto errout;
    }

    if (opa_json_writer_emit_value(w, v) != 0)
    {
        goto errout;
    }

    if (opa_json_writer_emit_char(w, 0) != 0)
    {
        goto errout;
    }

    return w->buf;

errout:
    opa_free(w->buf);
    return NULL;
}

char *opa_json_dump(opa_value *v)
{
    opa_json_writer w;
    opa_json_writer_init(&w);
    return opa_json_writer_write(&w, v);
}

char *opa_value_dump(opa_value *v)
{
    opa_json_writer w;
    opa_json_writer_init(&w);
    w.set_literals_enabled = 1;
    w.non_string_object_keys_enabled = 1;
    return opa_json_writer_write(&w, v);
}