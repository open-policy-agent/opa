#include <string.h>

#include "glob.h"
#include "re2/re2.h"
#include "malloc.h"
#include "str.h"
#include "unicode.h"

#include <vector>
#include "glob-lexer.h"

// The following is a re-implementation of lexer
// https://github.com/gobwas/glob/blob.
//
// The MIT License (MIT)
//
// Copyright (c) 2016 Sergey Kamardin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

const static int eof = 0;

const static int char_any           = '*';
const static int char_comma         = ',';
const static int char_single        = '?';
const static int char_escape        = '\\';
const static int char_range_open    = '[';
const static int char_range_close   = ']';
const static int char_terms_open    = '{';
const static int char_terms_close   = '}';
const static int char_range_not     = '!';
const static int char_range_between = '-';

lexer::lexer(const char *source, size_t n_)
    : data(source), pos(0), n(n_), error(NULL), tokens(), terms_level(0), has_rune(false), last_rune("", 0, eof) { }

lexer::~lexer()
{
    for (int i = 0; i < tokens.size(); i++)
    {
        delete(tokens[i]);
    }
}

void lexer::next(token *out)
{
    if (error)
    {
        *out = token(glob_lexer_token_error, error, strlen(error));
        return;
    }

    if (!tokens.empty())
    {
        *out = *tokens[0];
        for (int i = 1; i < tokens.size(); i++)
        {
            tokens[i - 1] = tokens[i];
        }
        tokens.pop_back();
        return;
    }

    fetch_item();
    next(out);
}

void lexer::peek(rune *out)
{
    if (pos == n)
    {
        *out = rune(&data[pos], 0, eof);
        return;
    }

    int len;
    int cp = opa_unicode_decode_utf8(data, pos, n, &len);
    if (cp < 0)
    {
        *out = rune(&data[pos], 0, eof);
        return;
    }

    *out = rune(&data[pos], len, cp);
}

void lexer::read(rune *out) {
    if (has_rune)
    {
        has_rune = false;
        seek(last_rune.n);
        *out = last_rune;
        return;
    }

    peek(&last_rune);
    seek(last_rune.n);

    *out = last_rune;
}

void lexer::unread()
{
    if (has_rune) {
        error = "could not unread rune";
        return;
    }

    seek(-last_rune.n);
    has_rune = true;
}

void lexer::fetch_item()
{
    rune r("", 0, eof);
    read(&r);

    if (r.cp == eof)
    {
        tokens.push_back(new token(glob_lexer_token_eof, r.s, 0));
    }
    else if (r.cp == char_terms_open)
    {
        terms_enter();
        tokens.push_back(new token(glob_lexer_token_terms_open, r.s, r.n));
    }
    else if (r.cp == char_comma && in_terms())
    {
        tokens.push_back(new token(glob_lexer_token_separator, r.s, r.n));
    }
    else if (r.cp == char_terms_close && in_terms())
    {
        tokens.push_back(new token(glob_lexer_token_terms_close, r.s, r.n));
        terms_leave();
    }
    else if (r.cp == char_range_open)
    {
        tokens.push_back(new token(glob_lexer_token_range_open, r.s, r.n));
        fetch_range();
    }
    else if (r.cp == char_single)
    {
        tokens.push_back(new token(glob_lexer_token_single, r.s, r.n));
    }
    else if (r.cp == char_any)
    {
        rune n("", 0, eof);
        read(&n);
        if (n.cp == char_any)
        {
            tokens.push_back(new token(glob_lexer_token_super, r.s, r.n + n.n));
        } else {
            unread();
            tokens.push_back(new token(glob_lexer_token_any, r.s, r.n));
        }
    }
    else
    {
        const int in_text_breakers[] = {char_single, char_any, char_range_open, char_terms_open, 0};
        const int in_terms_breakers[] = {char_single, char_any, char_range_open, char_terms_open, char_terms_close, char_comma, 0};

        unread();
        fetch_text(in_terms() ? in_terms_breakers : in_text_breakers);
    }
}

void lexer::fetch_range()
{
    bool want_hi = false;
    bool want_close = false;
    bool seen_not = false;
    while (true) {
        rune r("", 0, eof);
        read(&r);
        if (r.cp == eof) {
            error = "unexpected end of input";
            return;
        }

        if (want_close)
        {
            if (r.cp != char_range_close) {
                error = "expected close range character";
            } else {
                tokens.push_back(new token(glob_lexer_token_range_close, r.s, r.n));
            }
            return;
        }

        if (want_hi)
        {
            tokens.push_back(new token(glob_lexer_token_range_hi, r.s, r.n));
            want_close = true;
            continue;
        }

        if (!seen_not && r.cp == char_range_not)
        {
            tokens.push_back(new token(glob_lexer_token_not, r.s, r.n));
            seen_not = true;
            continue;
        }

        rune n("", 0, eof);
        peek(&n);
        if (n.cp == char_range_between)
        {
            seek(n.n);
            tokens.push_back(new token(glob_lexer_token_range_lo, r.s, r.n));
            tokens.push_back(new token(glob_lexer_token_range_between, n.s, n.n));
            want_hi = true;
            continue;
        }

        unread(); // unread first peek and fetch as text
        static const int breakers[] = {char_range_close, 0};
        fetch_text(breakers);
        want_close = true;
    }
}

void lexer::fetch_text(const int *breakers)
{
    opa_array_t *arr = opa_cast_array(opa_array());
    bool escaped = false;
    rune r("", 0, eof);
    const char *s;

    for (read(&r), s = r.s; r.cp != eof; read(&r)) {
        if (!escaped)
        {
            if (r.cp == char_escape) {
                escaped = true;

                size_t n = static_cast<size_t>(r.s - s);
                if (n > 0)
                {
                    opa_array_append(arr, opa_string(s, n));
                }

                s = r.s + 1;
                continue;
            }

            for (int i = 0; breakers[i] != 0; i++)
            {
                if (breakers[i] == r.cp)
                {
                    unread();
                    goto done;
                }
            }
        }

        escaped = false;
    }
 done:
    size_t n = static_cast<size_t>(r.s - s);
    if (n > 0)
    {
        opa_array_append(arr, opa_string(s, n));
    }

    if (arr->len == 0)
    {
        opa_free(arr);
        return;
    }

    n = 0;
    for (int i = 0; i < arr->len; i++)
    {
        n += opa_cast_string(arr->elems[i].v)->len;
    }

    char *v = static_cast<char *>(opa_malloc(n));
    for (int i = 0, j = 0; i < arr->len; i++)
    {
        opa_string_t *s = opa_cast_string(arr->elems[i].v);
        memcpy(&v[j], s->v, s->len);
        j += s->len;
    }

    opa_free(arr);
    tokens.push_back(new token(glob_lexer_token_text, v, n));
}
