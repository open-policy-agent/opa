#include "malloc.h"
#include "mpd.h"
#include "std.h"
#include "str.h"
#include "string.h"
#include "strings.h"
#include "unicode.h"

OPA_BUILTIN
opa_value *opa_strings_any_prefix_match(opa_value *a, opa_value *b)
{
    // If the first argument is a string, continue to matching.
    // Otherwise, if it's an array or set, recur for each element.
    // In other words if opa_strings_any_prefix_match(["test", "test2"], x) is called,
    // then this will result in two recurrent calls:
    // - opa_strings_any_prefix_match("test", x)
    // - opa_strings_any_prefix_match("test2", x)
    switch (opa_value_type(a))
    {
    case OPA_STRING: {
        break;
    }
    case OPA_ARRAY:
    case OPA_SET: {
        opa_value *prev = NULL;
        opa_value *curr = NULL;
        while ((curr = opa_value_iter(a, prev)) != NULL)
        {
            opa_value *elem = opa_value_get(a, curr);
            if (opa_value_type(elem) != OPA_STRING)
            {
                return NULL;
            }

            opa_value *res = opa_strings_any_prefix_match(elem, b);
            if (res == NULL) {
                return NULL;
            }
            opa_boolean_t *res_b = opa_cast_boolean(res);
            if (res_b->v) {
                return res;
            }
            opa_value_free(res);

            prev = curr;
        }
        return opa_boolean(false);
    }
    default:
        return NULL;
    }

    // If the second argument is a string, continue to matching.
    // Otherwise, if it's an array or set, recur for each element.
    // In other words if opa_strings_any_prefix_match(x, ["test", "test2"]) is called,
    // then this will result in two recurrent calls:
    // - opa_strings_any_prefix_match(x, "test")
    // - opa_strings_any_prefix_match(x, "test2")
    switch (opa_value_type(b))
    {
    case OPA_STRING: {
        break;
    }
    case OPA_ARRAY:
    case OPA_SET: {
        opa_value *prev = NULL;
        opa_value *curr = NULL;
        while ((curr = opa_value_iter(b, prev)) != NULL)
        {
            opa_value *elem = opa_value_get(b, curr);
            if (opa_value_type(elem) != OPA_STRING)
            {
                return NULL;
            }

            opa_value *res = opa_strings_any_prefix_match(a, elem);
            if (res == NULL) {
                return NULL;
            }
            opa_boolean_t *res_b = opa_cast_boolean(res);
            if (res_b->v) {
                return res;
            }
            opa_value_free(res);

            prev = curr;
        }
        return opa_boolean(false);
    }
    default:
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *prefix = opa_cast_string(b);

    if (s->len < prefix->len)
    {
        return opa_boolean(false);
    }

    return opa_boolean(opa_strncmp(s->v, prefix->v, prefix->len) == 0);
}

OPA_BUILTIN
opa_value *opa_strings_any_suffix_match(opa_value *a, opa_value *b)
{

    // If the first argument is a string, continue to matching.
    // Otherwise, if it's an array or set, recur for each element.
    // In other words if opa_strings_any_suffix_match(["test", "test2"], x) is called,
    // then this will result in two recurrent calls:
    // - opa_strings_any_suffix_match("test", x)
    // - opa_strings_any_suffix_match("test2", x)
    switch (opa_value_type(a))
    {
    case OPA_STRING: {
        break;
    }
    case OPA_ARRAY:
    case OPA_SET: {
        opa_value *prev = NULL;
        opa_value *curr = NULL;
        while ((curr = opa_value_iter(a, prev)) != NULL)
        {
            opa_value *elem = opa_value_get(a, curr);
            if (opa_value_type(elem) != OPA_STRING)
            {
                return NULL;
            }

            opa_value *res = opa_strings_any_suffix_match(elem, b);
            if (res == NULL) {
                return NULL;
            }
            opa_boolean_t *res_b = opa_cast_boolean(res);
            if (res_b->v) {
                return res;
            }
            opa_value_free(res);

            prev = curr;
        }
        return opa_boolean(false);
    }
    default:
        return NULL;
    }

    // If the second argument is a string, continue to matching.
    // Otherwise, if it's an array or set, recur for each element.
    // In other words if opa_strings_any_suffix_match(x, ["test", "test2"]) is called,
    // then this will result in two recurrent calls:
    // - opa_strings_any_suffix_match(x, "test")
    // - opa_strings_any_suffix_match(x, "test2")
    switch (opa_value_type(b))
    {
    case OPA_STRING: {
        break;
    }
    case OPA_ARRAY:
    case OPA_SET: {
        opa_value *prev = NULL;
        opa_value *curr = NULL;
        while ((curr = opa_value_iter(b, prev)) != NULL)
        {
            opa_value *elem = opa_value_get(b, curr);
            if (opa_value_type(elem) != OPA_STRING)
            {
                return NULL;
            }

            opa_value *res = opa_strings_any_suffix_match(a, elem);
            if (res == NULL) {
                return NULL;
            }
            opa_boolean_t *res_b = opa_cast_boolean(res);
            if (res_b->v) {
                return res;
            }
            opa_value_free(res);

            prev = curr;
        }
        return opa_boolean(false);
    }
    default:
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *suffix = opa_cast_string(b);

    if (s->len < suffix->len)
    {
        return opa_boolean(false);
    }

    for (int i = 0; i < suffix->len; i++)
    {
        if (s->v[s->len - suffix->len + i] != suffix->v[i])
        {
            return opa_boolean(false);
        }
    }

    return opa_boolean(true);
}

OPA_BUILTIN
opa_value *opa_strings_concat(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *join = opa_cast_string(a);
    size_t len = 1; // 1 for '\0'

    switch (opa_value_type(b))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(b);

        for (int i = 0; i < a->len; i++)
        {
            opa_value *v = a->elems[i].v;
            if (opa_value_type(v) != OPA_STRING) {
                return NULL;
            }

            len += opa_cast_string(v)->len;
        }

        if (a->len > 0)
        {
            len += (a->len - 1) * join->len;
        }

        char *str = opa_malloc(len);
        size_t j = 0;

        for (int i = 0; i < a->len; i++)
        {
            if (i > 0)
            {
                memcpy(&str[j], join->v, join->len);
                j += join->len;
            }

            opa_string_t *s = opa_cast_string(a->elems[i].v);
            memcpy(&str[j], s->v, s->len);
            j += s->len;
        }

        str[len - 1] = '\0';
        return opa_string_allocated(str, len - 1);
    }

    case OPA_SET: {
        opa_set_t *s = opa_cast_set(b);

        for (int i = 0; i < s->n; i++)
        {
            opa_set_elem_t *elem = s->buckets[i];

            while (elem != NULL)
            {
                opa_value *v = elem->v;
                if (opa_value_type(v) != OPA_STRING)
                {
                    return NULL;
                }

                len += opa_cast_string(v)->len;
                elem = elem->next;
            }
        }

        if (s->len > 0)
        {
            len += (s->len - 1) * join->len;
        }

        char *str = opa_malloc(len);
        int j = -1;

        for (int i = 0; i < s->n; i++)
        {
            opa_set_elem_t *elem = s->buckets[i];

            while (elem != NULL)
            {
                if (j < 0)
                {
                    j = 0; // no separator before the first element written.
                } else {
                    memcpy(&str[j], join->v, join->len);
                    j += join->len;
                }

                opa_string_t *s = opa_cast_string(elem->v);
                memcpy(&str[j], s->v, s->len);
                j += s->len;

                elem = elem->next;
            }
        }

        str[len - 1] = '\0';
        return opa_string_allocated(str, len - 1);
    }

    default:
        return NULL;
    }
}

static int strings_indexof(opa_string_t *s, int pos, opa_string_t *substr)
{
    // TODO: Implement Karp-Rabin string search.

    for (int i = pos; i <= (int)s->len - (int)substr->len; i++)
    {
        if (opa_strncmp(&s->v[i], substr->v, substr->len) == 0)
        {
            return i;
        }
    }

    return -1;
}

OPA_BUILTIN
opa_value *opa_strings_contains(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *substr = opa_cast_string(b);

    return opa_boolean(strings_indexof(s, 0, substr) >= 0);
}

OPA_BUILTIN
opa_value *opa_strings_endswith(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *suffix = opa_cast_string(b);

    if (s->len < suffix->len)
    {
        return opa_boolean(false);
    }

    for (int i = 0; i < suffix->len; i++)
    {
        if (s->v[s->len - suffix->len + i] != suffix->v[i])
        {
            return opa_boolean(false);
        }
    }

    return opa_boolean(true);
}

OPA_BUILTIN
opa_value *opa_strings_format_int(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_NUMBER || opa_value_type(b) != OPA_NUMBER)
    {
        return NULL;
    }

    opa_number_t *base = opa_cast_number(b);

    long long v;
    if (opa_number_try_int(base, &v) != 0)
    {
        return NULL;
    }

    const char *format;
    switch (v) {
    case 2:
        format = "%b";
        break;
    case 8:
        format = "%o";
        break;
    case 10:
        format = "%d";
        break;
    case 16:
        format = "%x";
        break;
    default:
        return NULL;
    }

    mpd_t *input = opa_number_to_bf(a);
    if (input == NULL)
    {
        return NULL;
    }

    mpd_t *i = mpd_qnew();
    uint32_t status = 0;
    mpd_qtrunc(i, input, mpd_max_ctx(), &status);
    if (status != 0)
    {
        opa_abort("strings: truncate failed");
    }

    int32_t w = mpd_qget_i32(i, &status);
    if (status != 0)
    {
        opa_abort("strings: get uint failed");
    }

    char *str = opa_malloc(21); // enough for int_t (with sign).

    if (w < 0)
    {
        str[0] = '-';
        snprintf(&str[1], 21, format, -w);
    } else {
        snprintf(str, 21, format, w);
    }

    return opa_string_allocated(str, opa_strlen(str));
}

OPA_BUILTIN
opa_value *opa_strings_indexof(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *substr = opa_cast_string(b);
    int n = strings_indexof(s, 0, substr);

    if (n < 0)
    {
        return opa_number_int(n);
    }

    int units = 0;
    for (int i = 0, len = 0; i < n; units++, i += len)
    {
        if (opa_unicode_decode_utf8(s->v, i, s->len, &len) == -1)
        {
            opa_abort("string: invalid unicode");
        }
    }

    return opa_number_int(units);
}

OPA_BUILTIN
opa_value *opa_strings_replace(opa_value *a, opa_value *b, opa_value *c)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING || opa_value_type(c) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *old = opa_cast_string(b);
    opa_string_t *new = opa_cast_string(c);

    int cap = s->len + 1;
    char *r = opa_malloc(cap);

    int j = 0;
    for (int i = 0; i < s->len; ) {
        int match = strings_indexof(s, i, old);
        int copy_len = match == -1 ? s->len - i : match - i;
        int new_r_len = j + copy_len + new->len + 1; // Optimistic allocation for new string.

        if (cap < new_r_len)
        {
            cap = new_r_len;
            r = opa_realloc(r, cap);
        }

        memcpy(&r[j], &s->v[i], copy_len);
        i += copy_len + old->len;
        j += copy_len;

        if (match != -1)
        {
            memcpy(&r[j], new->v, new->len);
            j += new->len;
        }
    }

    r[j] = '\0';

    return opa_string_allocated(r, j);
}

OPA_BUILTIN
opa_value *opa_strings_replace_n(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_OBJECT || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_object_t *old_new = opa_cast_object(a);
    opa_string_t *s = opa_cast_string(b);

    char *buf = opa_malloc(s->len + 1);
    memcpy(buf, s->v, s->len + 1);
    opa_value *result = opa_string_allocated(buf, s->len);

    for (int i = 0; i < old_new->n; i++)
    {
        opa_object_elem_t *elem = old_new->buckets[i];

        while (elem != NULL)
        {
            opa_value *old = elem->k;
            opa_value *new = elem->v;
            if (opa_value_type(old) != OPA_STRING || opa_value_type(new) != OPA_STRING)
            {
                opa_value_free(result);
                return NULL;
            }

            opa_value *r = opa_strings_replace(result, old, new);
            opa_value_free(result);
            result = r;

            elem = elem->next;
        }
    }

    return result;
}

OPA_BUILTIN
opa_value *opa_strings_reverse(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);

    char *reversed = opa_malloc(s->len + 1);

    for (int i = 0; i < s->len; )
    {
        int len = 0;
        if (opa_unicode_decode_utf8(s->v, i, s->len, &len) == -1)
        {
            opa_abort("string: invalid unicode");
        }
        memcpy(&reversed[s->len - i - len], &s->v[i], len);
        i += len;
    }
    reversed[s->len] = '\0';

    return opa_string_allocated(reversed, s->len);
}

OPA_BUILTIN
opa_value *opa_strings_split(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *d = opa_cast_string(b);
    opa_array_t *arr = opa_cast_array(opa_array());

    if (d->len == 0)
    {
        // Split at UTF-8 character boundaries.
        for (int i = 0; i < s->len; )
        {
            int len = 0;
            if (opa_unicode_decode_utf8(s->v, i, s->len, &len) == -1)
            {
                opa_abort("string: invalid unicode");
            }

            char *str = opa_malloc(len + 1);
            memcpy(str, &s->v[i], len);
            str[len] = '\0';

            opa_array_append(arr, opa_string_allocated(str, len));

            i += len;
        }

        return &arr->hdr;
    }

    int j = 0;
    for (int i = 0; s->len >= d->len && i <= (s->len - d->len); )
    {
        if (opa_strncmp(&s->v[i], d->v, d->len) == 0)
        {
            char *str = opa_malloc(i - j + 1);
            memcpy(str, &s->v[j], i - j);
            str[i - j] = '\0';

            opa_array_append(arr, opa_string_allocated(str, i - j));

            i += d->len;
            j = i;
        }
        else
        {
            i++;
        }
    }

    char *str = opa_malloc(s->len - j + 1);
    memcpy(str, &s->v[j], s->len - j);
    str[s->len - j] = '\0';

    opa_array_append(arr, opa_string_allocated(str, s->len - j));

    return &arr->hdr;
}

OPA_BUILTIN
opa_value *opa_strings_startswith(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *prefix = opa_cast_string(b);

    if (s->len < prefix->len)
    {
        return opa_boolean(false);
    }

    return opa_boolean(opa_strncmp(s->v, prefix->v, prefix->len) == 0);
}

OPA_BUILTIN
opa_value *opa_strings_substring(opa_value *a, opa_value *b, opa_value *c)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_NUMBER || opa_value_type(c) != OPA_NUMBER)
    {
        return NULL;
    }

    opa_string_t *base = opa_cast_string(a);

    long long start, length;
    if (opa_number_try_int(opa_cast_number(b), &start))
    {
        return NULL;
    }

    if (opa_number_try_int(opa_cast_number(c), &length))
    {
        return NULL;
    }

    if (start < 0)
    {
        return NULL;
    }

    if (length == 0)
    {
        return opa_string_terminated("");
    }

    size_t spos = base->len, epos = base->len;
    for (int i = 0, units = 0, len = 0; i < base->len; units++, i += len)
    {
        if (units == start)
        {
            spos = i;
        }

        if (opa_unicode_decode_utf8(base->v, i, base->len, &len) == -1)
        {
            opa_abort("string: invalid unicode");
        }

        if (units < start)
        {
            // Start index not reached yet.
            continue;
        }

        if (length < 0)
        {
            // Everything from start to end.
            break;
        }

        if (length == (units - start))
        {
            epos = i;
            break;
        }
    }

    char *str = opa_malloc(epos - spos + 1);
    memcpy(str, &base->v[spos], epos - spos);
    str[epos - spos] = 0;
    return opa_string_allocated(str, epos - spos);
}

OPA_BUILTIN
opa_value *opa_strings_trim(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_value *s = opa_strings_trim_left(a, b);
    opa_value *r = opa_strings_trim_right(s, b);
    opa_value_free(s);
    return r;
}

OPA_BUILTIN
opa_value *opa_strings_trim_left(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *l = opa_cast_string(b);

    int j = 0;
    while (j < s->len)
    {
        int i = 0;
        while (i < l->len)
        {
            int len;
            if (opa_unicode_decode_utf8(l->v, i, l->len, &len) == -1)
            {
                opa_abort("string: invalid unicode");
            }

            if ((j + len) <= s->len && opa_strncmp(&l->v[i], &s->v[j], len) == 0)
            {
                j += len; // trim codepoint.
                break;
            }

            i += len;
        }

        if (i == l->len) {
            // Nothing to trim.
            break;
        }
    }

    char *str = opa_malloc(s->len - j + 1);
    memcpy(str, &s->v[j], s->len - j + 1);
    return opa_string_allocated(str, s->len - j);
}

OPA_BUILTIN
opa_value *opa_strings_trim_prefix(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *pre = opa_cast_string(b);
    int start = 0;

    if (s->len >= pre->len && opa_strncmp(s->v, pre->v, pre->len) == 0)
    {
        start = pre->len;
    }

    const int len = s->len - start;
    char *str = opa_malloc(len + 1);
    memcpy(str, &s->v[start], len + 1);
    return opa_string_allocated(str, len);
}

OPA_BUILTIN
opa_value *opa_strings_trim_right(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *r = opa_cast_string(b);

    int j = s->len;
    while (j > 0)
    {
        int last = opa_unicode_last_utf8(s->v, 0, j);
        if (last == -1)
        {
            opa_abort("string: invalid unicode");
        }

        int i = 0;
        while (i < r->len)
        {
            int len;
            if (opa_unicode_decode_utf8(r->v, i, r->len, &len) == -1)
            {
                opa_abort("string: invalid unicode");
            }

            if ((last + len) <= s->len && opa_strncmp(&r->v[i], &s->v[last], len) == 0)
            {
                j -= len; // trim codepoint.
                break;
            }

            i += len;
        }

        if (i == r->len) {
            // Nothing to trim.
            break;
        }
    }

    char *str = opa_malloc(j + 1);
    memcpy(str, s->v, j);
    str[j] = '\0';
    return opa_string_allocated(str, j);
}

OPA_BUILTIN
opa_value *opa_strings_trim_suffix(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    opa_string_t *suf = opa_cast_string(b);
    int len = s->len;

    if (s->len >= suf->len && opa_strncmp(&s->v[s->len - suf->len], suf->v, suf->len) == 0)
    {
        len -= suf->len;
    }

    char *str = opa_malloc(len + 1);
    memcpy(str, s->v, len);
    str[len] = '\0';
    return opa_string_allocated(str, len);
}

static opa_value *trim_space(const char *s, int start, int end)
{
    while (start < end)
    {
        int len = 0;
        int cp = opa_unicode_decode_utf8(s, start, end, &len);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        if (!opa_unicode_is_space(cp))
        {
            break;
        }

        start += len;
    }

    while (start < end)
    {
        int last = opa_unicode_last_utf8(s, start, end);
        if (last == -1)
        {
            opa_abort("string: invalid unicode");
        }

        int len;
        int cp = opa_unicode_decode_utf8(s, last, end, &len);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        if (!opa_unicode_is_space(cp))
        {
            break;
        }

        end = last;
    }

    char *str = opa_malloc(end - start + 1);
    memcpy(str, &s[start], end - start);
    str[end - start] = '\0';
    return opa_string_allocated(str, end - start);
}

OPA_BUILTIN
opa_value *opa_strings_trim_space(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);

    int start = 0;
    for (; start < s->len; start++)
    {
        unsigned char c = s->v[start];
        if (c >= 0x80) {
            // If we run into a non-ASCII byte, fall back to the
            // slower unicode-aware method on the remaining bytes
            return trim_space(s->v, start, s->len);
        }

        if (!(c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' || c == ' '))
        {
            break;
        }
    }

    int stop = s->len;
    for (; stop > start; stop--) {
        unsigned char c = s->v[stop-1];
        if (c >= 0x80)
        {
            return trim_space(s->v, start, stop);
        }

        if (!(c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' || c == ' '))
        {
            break;
        }
    }

    char *str = opa_malloc(stop - start + 1);
    memcpy(str, &s->v[start], stop - start);
    str[stop - start] = '\0';
    return opa_string_allocated(str, stop - start);
}

OPA_BUILTIN
opa_value *opa_strings_lower(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    int is_ascii = true;

    for (int i = 0; i < s->len && is_ascii; i++)
    {
        unsigned char c = s->v[i];
        is_ascii = c < 0x80;
    }

    if (is_ascii)
    {
        char *str = opa_malloc(s->len + 1);

        for (int i = 0; i < s->len; i++)
        {
            unsigned char c = s->v[i];
            if ('A' <= c && c <= 'Z')
            {
                c += 'a' - 'A';
            }

            str[i] = c;
        }

        str[s->len] = '\0';
        return opa_string_allocated(str, s->len);
    }

    int j = 0;
    char *out = malloc(s->len + 1);
    int cap = s->len;

    for (int i = 0; i < s->len; )
    {
        int len;
        int cp = opa_unicode_decode_utf8(s->v, i, s->len, &len);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        cp = opa_unicode_to_lower(cp);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        if (cap < (j + 4)) // Space for the longest possible UTF-8 character (4 bytes).
        {
            cap *= 2;

            if (cap < (j + 4))
            {
                cap = j + 4;
            }

            out = opa_realloc(out, cap + 1);
        }

        j += opa_unicode_encode_utf8(cp, &out[j]);
        i += len;
    }

    out[j] = '\0';
    return opa_string_allocated(out, j);
}

OPA_BUILTIN
opa_value *opa_strings_upper(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    int is_ascii = true;

    for (int i = 0; i < s->len && is_ascii; i++)
    {
        unsigned char c = s->v[i];
        is_ascii = c < 0x80;
    }

    if (is_ascii)
    {
        char *str = opa_malloc(s->len + 1);

        for (int i = 0; i < s->len; i++)
        {
            unsigned char c = s->v[i];
            if ('a' <= c && c <= 'z')
            {
                c -= 'a' - 'A';
            }

            str[i] = c;
        }

        str[s->len] = '\0';
        return opa_string_allocated(str, s->len);
    }

    int j = 0;
    char *out = malloc(s->len + 1);
    int cap = s->len;

    for (int i = 0; i < s->len; )
    {
        int len;
        int cp = opa_unicode_decode_utf8(s->v, i, s->len, &len);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        cp = opa_unicode_to_upper(cp);
        if (cp == -1)
        {
            opa_abort("string: invalid unicode");
        }

        if (cap < (j + 4)) // Space for the longest possible UTF-8 character (4 bytes).
        {
            cap *= 2;

            if (cap < (j + 4))
            {
                cap = j + 4;
            }

            out = opa_realloc(out, cap + 1);
        }

        j += opa_unicode_encode_utf8(cp, &out[j]);
        i += len;
    }

    out[j] = '\0';
    return opa_string_allocated(out, j);
}
