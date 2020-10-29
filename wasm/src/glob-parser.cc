#include <string>
#include <vector>

#include "glob-lexer.h"
#include "glob-parser.h"
#include "unicode.h"

// The following is a re-implementation of parser in
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

node::~node()
{
    for (int i = 0; i < children.size(); i++)
    {
        delete children[i];
    }
}

node* node::insert(node *child)
{
    children.push_back(child);
    child->parent = this;
    return this;
}

bool node::equal(node *other)
{
    if (kind != other->kind ||
        text != other->text ||
        lo != other->lo ||
        hi != other->hi ||
        not_ != other->not_)
    {
        return false;
    }

    if (children.size() != other->children.size())
    {
        return false;
    }

    for (int i = 0; i < children.size(); i++)
    {
        if (!children[i]->equal(other->children[i]))
        {
            return false;
        }
    }

    return true;
}

struct state;
typedef void (*parser)(state *s, lexer *lexer);

struct state {
    node *tree;
    parser parser;
    std::string error;
};

static void parser_main(state *s, lexer *lexer);
static void parser_range(state *s, lexer *lexer);

std::string glob_parse(lexer *lexer, node **output)
{
    node *root = new node(kind_pattern);

    for (struct state s = {root, parser_main}; s.parser; )
    {
        s.parser(&s, lexer);
        if (s.error != "")
        {
            delete root;
            return s.error;
        }
    }

    *output = root;
    return "";
}

static void parser_main(state *s, lexer *lexer) {
    token token(glob_lexer_token_eof, "", 0);
    lexer->next(&token);

    switch (token.kind)
    {
    case glob_lexer_token_eof:
        s->parser = NULL;
        break;

    case glob_lexer_token_error:
        s->parser = NULL;
        s->error = token.s;
        break;

    case glob_lexer_token_text:
        s->tree->insert(new node(kind_text, token.s));
        s->parser = parser_main;
        break;

    case glob_lexer_token_any:
        s->tree->insert(new node(kind_any));
        s->parser = parser_main;
        break;

    case glob_lexer_token_super:
        s->tree->insert(new node(kind_super));
        s->parser = parser_main;
        break;

    case glob_lexer_token_single:
        s->tree->insert(new node(kind_single));
        s->parser = parser_main;
        break;

    case glob_lexer_token_range_open:
        s->parser = parser_range;
        break;

    case glob_lexer_token_terms_open: {
        node *a = new node(kind_any_of);
        s->tree->insert(a);

        node *p = new node(kind_pattern);
        a->insert(p);

        s->tree = p;
        s->parser = parser_main;
        break;
    }

    case glob_lexer_token_separator: {
        node *p = new node(kind_pattern);
        s->tree->parent->insert(p);
        s->tree = p;
        s->parser = parser_main;
        break;
    }

    case glob_lexer_token_terms_close:
        s->tree = s->tree->parent->parent;
        s->parser = parser_main;
        break;

    default:
        s->parser = NULL;
        s->error = "unexpected token";
        break;
    }
}

static void parser_range(state *s, lexer *lexer)
{
    bool not_ = false;
    std::string lo, hi;
    int lo_cp = 0, hi_cp = 0;
    std::string chars;

    while (true)
    {
        token token(glob_lexer_token_eof, "", 0);
        lexer->next(&token);

        switch (token.kind)
        {
        case glob_lexer_token_eof:
            s->error = "unexpected end";
            s->parser = NULL;
            return;

        case glob_lexer_token_error:
            s->parser = NULL;
            s->error = token.s;
            return;

        case glob_lexer_token_not:
            not_ = true;
            break;

        case glob_lexer_token_range_lo: {
            int len;
            lo_cp = opa_unicode_decode_utf8(token.s.c_str(), 0, token.s.length(), &len);
            if (lo_cp < 0 || len != token.s.length())
            {
                s->parser = NULL;
                s->error = "unexpected length of lo character";
                return;
            }

            lo = token.s;
            break;
        }

        case glob_lexer_token_range_between:
            break;

        case glob_lexer_token_range_hi: {
            int len;
            hi_cp = opa_unicode_decode_utf8(token.s.c_str(), 0, token.s.length(), &len);
            if (hi_cp < 0 || len != token.s.length())
            {
                s->parser = NULL;
                s->error = "unexpected length of hi character";
                return;
            }

            hi = token.s;

            if (hi < lo)
            {
                s->parser = NULL;
                s->error = "hi character should be greater than lo character";
                return;
            }
            break;
        }

        case glob_lexer_token_text:
            chars = token.s;
            break;

        case glob_lexer_token_range_close: {
            const bool is_range = lo_cp != 0 && hi_cp != 0;
            const bool is_chars = chars != "";

            if (is_chars == is_range)
            {
                s->parser = NULL;
                s->error = "could not parse range";
                return;
            }

            if (is_range)
            {
                s->tree->insert(new node(kind_range, lo, hi, not_));
            } else {
                s->tree->insert(new node(kind_list, chars, not_));
            }

            s->parser = parser_main;
            return;
        }
        }
    }
}
