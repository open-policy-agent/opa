#ifndef OPA_GLOB_LEXER_H
#define OPA_GLOB_LEXER_H

#include <string>
#include <vector>

enum token_kind {
    glob_lexer_token_eof = 0,
    glob_lexer_token_error = 1,
    glob_lexer_token_text = 2,
    glob_lexer_token_char = 3,
    glob_lexer_token_any = 4,
    glob_lexer_token_super = 5,
    glob_lexer_token_single = 6,
    glob_lexer_token_not = 7,
    glob_lexer_token_separator = 8,
    glob_lexer_token_range_open = 9,
    glob_lexer_token_range_close = 10,
    glob_lexer_token_range_lo = 11,
    glob_lexer_token_range_hi = 12,
    glob_lexer_token_range_between = 13,
    glob_lexer_token_terms_open = 14,
    glob_lexer_token_terms_close = 15
};

class rune {
public:
    inline rune(const char *s, size_t n, int cp_) : s(s), n(n), cp(cp_) { }
    const char *s;
    size_t n;
    int cp;
};

class token {
public:
    inline token(int kind_, const char *s_, size_t n) : kind(kind_), s(s_, n) { }
    int kind;
    std::string s;
};

class lexer
{
public:
    lexer(const char *source, size_t n);
    ~lexer();
    void next(token *token);
private:
    void peek(rune *r);
    void read(rune *r);
    inline void seek(int w) {  pos += w; }
    void unread();
    inline bool in_terms() { return terms_level > 0; }
    inline void terms_enter() { terms_level++; }
    inline void terms_leave() { terms_level--; }
    void fetch_item();
    void fetch_range();
    void fetch_text(const int *breakers);

    const char* data;
    size_t pos;
    size_t n;
    const char *error;
    std::vector<token*> tokens;
    int terms_level;
    bool has_rune;
    rune last_rune;
};

#endif
