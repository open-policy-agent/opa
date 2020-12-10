#include <string>
#include <unordered_map>

#include "glob.h"
#include "glob-compiler.h"
#include "malloc.h"
#include "regex.h"
#include "value.h"

struct cache_key {
public:
    inline cache_key() : pattern(""), delimiters() { }
    inline cache_key(const std::string& pattern_, const std::vector<std::string>& delimiters_) : pattern(pattern_), delimiters(delimiters_) { }
    inline bool operator==(const cache_key& key) const {
        return pattern == key.pattern && delimiters == key.delimiters;
    }
    std::string pattern;
    std::vector<std::string> delimiters;
};

template <>
struct std::hash<cache_key>
{
    size_t operator()(const cache_key& key) const
    {
        std::hash<std::string> hasher;
        size_t seed = hasher(key.pattern);

        for (int i = 0; i < key.delimiters.size(); i++)
        {
            seed ^= hasher(key.delimiters[i]) + 0x9e3779b9 + (seed<<6) + (seed>>2);
        }

        return seed;
    }
};

typedef std::unordered_map<cache_key, std::string> glob_cache;

static glob_cache* cache()
{
    glob_cache* c = static_cast<glob_cache*>(opa_builtin_cache_get(1));
    if (c == NULL)
    {
        c = new glob_cache();
        opa_builtin_cache_set(1, c);
    }

    return c;
}

opa_value *opa_glob_match(opa_value *pattern, opa_value *delimiters, opa_value *match)
{
    if (opa_value_type(pattern) != OPA_STRING || opa_value_type(delimiters) != OPA_ARRAY || opa_value_type(match) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *p = opa_cast_string(pattern);
    opa_array_t *d = opa_cast_array(delimiters);

    std::vector<std::string> v;

    for (int i = 0; i < d->len; i++)
    {
        if (opa_value_type(d->elems[i].v) != OPA_STRING)
        {
            return NULL;
        }

        opa_string_t *s = opa_cast_string(d->elems[i].v);
        std::string delimiter(s->v, s->len);
        v.push_back(delimiter);
    }

    glob_cache *c = cache();
    cache_key key = cache_key(std::string(p->v, p->len), v);
    glob_cache::iterator i = c->find(key);
    std::string re2;
    if (i != c->end())
    {
        re2 = i->second;
    } else {
        std::string error = glob_translate(p->v, p->len, v, &re2);
        if (!error.empty())
        {
            return NULL;
        }

        cache()->insert(std::make_pair(key, re2));
    }

    return opa_regex_match(opa_string(re2.c_str(), re2.length()), match);
}
