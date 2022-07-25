#include "glob-parser.h"
#include "unicode.h"
#include "std.h"

static const char special_characters[] = ".,:\"=<>[]^/\\{}|*+?";

// escape any special re2 characters in a text.
static std::string escape(const std::string& s)
{
    std::string x;
    for (int i = 0; i < s.length(); i++)
    {
        unsigned char c = s[i];
        for (int j = 0; special_characters[j] != '\0'; j++)
        {
            if (special_characters[j] == s[i])
            {
                x += '\\';
            }
        }

        x += c;
    }

    return x;
}

std::string node::re2(const std::string& single_mark)
{
    std::string s;

    if (parent == NULL) {
        s = "^";
    }

    switch (kind)
    {
    case kind_pattern:
        for (int i = 0; i < children.size(); i++)
        {
            s += children[i]->re2(single_mark);
        }
        break;

    case kind_list:
        s += "[";

        if (not_)
        {
            s += "^";
        }

        s += escape(text);
        s += "]";

        break;

    case kind_range:
        s += "[";

        if (not_)
        {
            s += "^";
        }
        s += lo;
        s += "-";
        s += hi;

        s += "]";

        break;

    case kind_text:
        s += escape(text);
        break;

    case kind_any:
        s += single_mark + "*";
        break;

    case kind_super:
        s += ".*";
        break;

    case kind_single:
        s += single_mark;
        break;

    case kind_any_of:
        s += "(";

        for (int i = 0; i < children.size(); i++)
        {
            if (i > 0)
            {
                s += "|";
            }
            s += children[i]->re2(single_mark);
        }

        s += ")";
        break;

    default:
        break;
    }

    if (parent == NULL)
    {
        s += "$";
    }

    return s;
}

std::string glob_translate(const char *glob, size_t n, const std::vector<std::string>& delimiters, std::string *re2)
{
    lexer *l = new lexer(glob, n);
    node *root = NULL;
    std::string error = glob_parse(l, &root);
    if (error != "")
    {
        delete l;
        return error;
    }

    std::string single_mark;

    if (delimiters.empty())
    {
        single_mark = ".";
    } else {
        single_mark = "[^";

        for (int i = 0; i < delimiters.size(); i++)
        {
            int len;
            if (opa_unicode_decode_utf8(delimiters[i].c_str(), 0, delimiters[i].length(), &len) < 0 || len != delimiters[i].length())
            {
                return "delimiter is not a single character";
            }

            single_mark += escape(delimiters[i]);
        }

        single_mark += "]";
    }

    *re2 = root->re2(single_mark);
    delete(root);
    delete(l);
    return "";
}

