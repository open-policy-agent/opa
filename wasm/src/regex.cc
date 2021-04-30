#include "regex.h"
#include "re2/re2.h"
#include "malloc.h"
#include "std.h"
#include "str.h"
#include "unicode.h"
#include "util/utf.h"

#include <unordered_map>

typedef std::unordered_map<std::string, re2::RE2*> re_cache;

OPA_BUILTIN
opa_value *opa_regex_is_valid(opa_value *pattern)
{
    if (opa_value_type(pattern) != OPA_STRING)
    {
        return opa_boolean(false);
    }

    std::string pat(opa_cast_string(pattern)->v, opa_cast_string(pattern)->len);
    re2::RE2::Options options;
    re2::RE2 re(pat, options);
    return opa_boolean(re.ok());
}

static re_cache* cache()
{
    re_cache* c = static_cast<re_cache*>(opa_builtin_cache_get(0));
    if (c == NULL)
    {
        c = new re_cache();
        opa_builtin_cache_set(0, c);
    }

    return c;
}

// compile compiles a pattern, using an earlier compilation if possible.
static re2::RE2* compile(const char *pattern)
{
    re_cache* c = cache();
    re_cache::iterator i = c->find(pattern);
    if (i != c->end())
    {
        return i->second;
    }

    re2::RE2::Options options;
    options.set_log_errors(false);
    re2::RE2 *re = new re2::RE2(pattern, options);
    if (!re->ok())
    {
        delete(re);
        return NULL;
    }

    return re;
}

// reuse returns the precompiled pattern to the cache.
static void reuse(re2::RE2 *re)
{
    cache()->insert(std::make_pair(re->pattern(), re));
}

OPA_BUILTIN
opa_value *opa_regex_match(opa_value *pattern, opa_value *value)
{
    if (opa_value_type(pattern) != OPA_STRING || opa_value_type(value) != OPA_STRING)
    {
        return NULL;
    }
    std::string pat(opa_cast_string(pattern)->v, opa_cast_string(pattern)->len);
    re2::RE2* re = compile(pat.c_str());
    if (re == NULL)
    {
        // TODO: return an error.
        return NULL;
    }

    std::string v(opa_cast_string(value)->v, opa_cast_string(value)->len);
    bool match = re2::RE2::PartialMatch(v, *re);

    reuse(re);
    return opa_boolean(match);
}

OPA_BUILTIN
opa_value *opa_regex_find_all_string_submatch(opa_value *pattern, opa_value *value, opa_value *number)
{
    if (opa_value_type(pattern) != OPA_STRING || opa_value_type(value) != OPA_STRING || opa_value_type(number) != OPA_NUMBER)
    {
        return NULL;
    }

    long long num_results;
    if (opa_number_try_int(opa_cast_number(number), &num_results))
    {
        return NULL;
    }

    std::string pat(opa_cast_string(pattern)->v, opa_cast_string(pattern)->len);
    re2::RE2* re = compile(pat.c_str());
    if (re == NULL)
    {
        // TODO: return an error.
        return NULL;
    }

    opa_string_t *s = opa_cast_string(value);
    opa_array_t *result = opa_cast_array(opa_array());
    int nsubmatch = re->NumberOfCapturingGroups() + 1;
    re2::StringPiece submatches[nsubmatch];

    // The following is effectively refactored RE2::GlobalReplace:

    const char* p = s->v;
    const char* ep = p + s->len;
    const char* lastend = NULL;
    int pos = 0;

    while (p <= ep && (num_results == -1 || result->len < num_results)) {
        if (!re->Match(s->v, static_cast<size_t>(p - s->v), s->len, re2::RE2::UNANCHORED, submatches, nsubmatch))
        {
            break;
        }

        if (p < submatches[0].data())
        {
            pos += submatches[0].data() - p;
        }

        if (submatches[0].data() == lastend && submatches[0].empty()) {
            // Disallow empty match at end of last match: skip ahead.
            //
            // fullrune() takes int, not ptrdiff_t. However, it just looks
            // at the leading byte and treats any length >= 4 the same.
            if (re->options().encoding() == RE2::Options::EncodingUTF8 &&
                re2::fullrune(p, static_cast<int>(std::min(ptrdiff_t{4}, ep - p)))) {
                // re is in UTF-8 mode and there is enough left of str
                // to allow us to advance by up to UTFmax bytes.
                re2::Rune r;
                int n = re2::chartorune(&r, p);
                // Some copies of chartorune have a bug that accepts
                // encodings of values in (10FFFF, 1FFFFF] as valid.
                if (r > re2::Runemax) {
                    n = 1;
                    r = re2::Runeerror;
                }

                if (!(n == 1 && r == re2::Runeerror)) {  // no decoding error
                    pos += n;
                    p += n;
                    continue;
                }
            }

            // Most likely, re is in Latin-1 mode. If it is in UTF-8 mode,
            // we fell through from above and the GIGO principle applies.
            pos++;
            p++;
            continue;
        }

        opa_array_t *r = opa_cast_array(opa_array_with_cap(nsubmatch));

        for (int i = 0; i < nsubmatch; i++) {
            const size_t length = submatches[i].length();
            char *str = (char *)opa_malloc(length + 1);

            memcpy(str, submatches[i].data(), length);
            str[length] = '\0';
            opa_array_append(r, opa_string_allocated(str, length));
        }

        opa_array_append(result, &r->hdr);

        p = submatches[0].data() + submatches[0].size();
        lastend = p;
    }

    reuse(re);
    return &result->hdr;
}
