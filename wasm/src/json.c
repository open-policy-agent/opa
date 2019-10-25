#include "string.h"
#include "value.h"
#include "json.h"
#include "malloc.h"
#include "printf.h"

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

    while (1)
    {
        if (opa_json_lex_eof(ctx))
        {
            goto err;
        }

        char b = *ctx->curr;

        switch (b)
        {
        case '\\':
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

        case '"':
            goto out;

        default:
            if (b < ' ' || b > '~')
            {
                goto err;
            }
            ctx->curr++;
            break;
        }
    }
out:
    ctx->buf_end = ctx->curr++;
    return OPA_JSON_TOKEN_STRING;

err:
    return OPA_JSON_TOKEN_ERROR;
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
}

long long opa_json_parse_int(const char *buf, int len)
{
    int i = 0;
    int sign = 1;

    if (buf[i] == '-')
    {
        sign = -1;
        i++;
    }

    long long n = 0;

    for (; i < len; i++)
    {
        n = (n * 10) + (long long)(buf[i] - '0');
    }

    return n * sign;
}

double opa_json_parse_float(const char *buf, int len)
{
    double sign = 1.0;
    int i = 0;

    if (buf[i] == '-')
    {
        sign = -1.0;
        i++;
    }

    // Handle integer component.
    double d = 0.0;

    for (; i < len && opa_isdigit(buf[i]); i++)
    {
        d = (10.0 * d) + (double)(buf[i] - '0');
    }

    d *= sign;

    if (i == len)
    {
        return d;
    }

    // Handle fraction component.
    if (buf[i] == '.')
    {
        i++;

        double b = 0.1;
        double frac = 0;

        for (; i < len && opa_isdigit(buf[i]); i++)
        {
            frac += b * (buf[i] - '0');
            b /= 10.0;
        }

        d += (frac * sign);

        if (i == len)
        {
            return d;
        }
    }

    // Handle exponent component.
    i++;
    int exp_sign = 1;

    if (buf[i] == '-')
    {
        exp_sign = -1;
        i++;
    }
    else if (buf[i] == '+')
    {
        i++;
    }

    int e = 0;

    for (; i < len; i++)
    {
        e = 10 * e + (int)(buf[i] - '0');
    }

    // Calculate pow(10, e).
    int x = 1;

    for (; e > 0; e--)
    {
        x *= 10;
    }

    return d * (double)(exp_sign * x);
}

opa_value *opa_json_parse_number(const char *buf, int len)
{
    for (int i = 0; i < len; i++)
    {
        if (buf[i] == '.' || buf[i] == 'e' || buf[i] == 'E')
        {
            return opa_number_float(opa_json_parse_float(buf, len));
        }
    }

    return opa_number_int(opa_json_parse_int(buf, len));
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

opa_value *opa_json_parse_object(opa_json_lex *ctx)
{
    opa_value *ret = opa_object();
    opa_object_t *obj = opa_cast_object(ret);
    int sep = 0;

    while (1)
    {
        int token = opa_json_lex_read(ctx);

        switch (token)
        {
        case OPA_JSON_TOKEN_OBJECT_END:
            return ret;
        case OPA_JSON_TOKEN_COMMA:
            if (sep)
            {
                sep = 0;
                continue;
            }
        }

        opa_value *key = opa_json_parse_token(ctx, token);

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
        opa_value *value = opa_json_parse_token(ctx, token);

        if (value == NULL)
        {
            return NULL;
        }

        opa_object_insert(obj, key, value);
        sep = 1;
    }
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
        return opa_string(ctx->buf, ctx->buf_end - ctx->buf);
    case OPA_JSON_TOKEN_ARRAY_START:
        return opa_json_parse_array(ctx);
    case OPA_JSON_TOKEN_OBJECT_START:
        return opa_json_parse_object(ctx);
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

typedef struct {
    char *buf;
    char *next;
    size_t len;
} opa_json_writer;

void opa_json_writer_init(opa_json_writer *w)
{
    w->buf = NULL;
    w->next = NULL;
    w->len = 0;
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

    return opa_json_writer_emit_chars(w, bs, sizeof(bs));
}

int opa_json_writer_emit_null(opa_json_writer *w)
{
    char bs[] = "null";

    return opa_json_writer_emit_chars(w, bs, sizeof(bs));
}

int opa_json_writer_emit_boolean(opa_json_writer *w, opa_boolean_t *b)
{
    if (b->v == 0)
    {
        char bs[] = "false";

        return opa_json_writer_emit_chars(w, bs, sizeof(bs));
    }

    char bs[] = "true";

    return opa_json_writer_emit_chars(w, bs, sizeof(bs));
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
    if (n->is_float)
    {
        return opa_json_writer_emit_float(w, n->v.f);
    }

    return opa_json_writer_emit_integer(w, n->v.i);
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
        if (s->v[i] == '"')
        {
            rc = opa_json_writer_emit_char(w, '\\');

            if (rc != 0)
            {
                return rc;
            }
        }

        rc = opa_json_writer_emit_char(w, s->v[i]);

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
    int rc = opa_json_writer_emit_value(w, k);

    if (rc != 0)
    {
        return rc;
    }

    rc = opa_json_writer_emit_char(w, ':');

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
        return opa_json_writer_emit_collection(w, v, '[', ']', opa_json_writer_emit_set_element);
    case OPA_OBJECT:
        return opa_json_writer_emit_collection(w, v, '{', '}', opa_json_writer_emit_object_element);
    }

    return -2;
}

const char *opa_json_dump(opa_value *v)
{
    opa_json_writer w;

    opa_json_writer_init(&w);

    if (opa_json_writer_grow(&w, 1024, 0) != 0)
    {
        goto errout;
    }

    if (opa_json_writer_emit_value(&w, v) != 0)
    {
        goto errout;
    }

    if (opa_json_writer_emit_char(&w, 0) != 0)
    {
        goto errout;
    }

    return w.buf;

errout:
    opa_free(w.buf);
    return NULL;
}