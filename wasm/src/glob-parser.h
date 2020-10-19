#ifndef OPA_GLOB_PARSER
#define OPA_GLOB_PARSER

#include <string>
#include "glob-lexer.h"

enum kind {
    kind_nothing = 0,
	kind_pattern = 1,
	kind_list = 2,
	kind_range = 3,
	kind_text = 4,
	kind_any = 5,
	kind_super = 6,
	kind_single = 7,
	kind_any_of = 8,
};

class node {
public:
    inline node(kind kind_) : kind(kind_), parent(NULL), children(), text(""), lo(""), hi(""), not_(false) { }
    inline node(kind kind_, const std::string& text_) : kind(kind_), parent(NULL), children(), text(text_), lo(""), hi(""), not_(false) { }
    inline node(kind kind_, const std::string& lo_, const std::string&  hi_, bool not__) : kind(kind_), parent(NULL), children(), text(""), lo(lo_), hi(hi_), not_(not__) { }
    inline node(kind kind_, const std::string& chars_, bool not__) : kind(kind_), parent(NULL), children(), text(chars_), lo(""), hi(""), not_(not__) { }
    ~node();
    node* insert(node *child);
    bool equal(node *other);
    std::string re2(const std::string& single_mark);

    kind kind;
    node *parent;
    std::vector<node*> children;
    std::string text;
    std::string lo;
    std::string hi;
    bool not_;
};

std::string glob_parse(lexer *lexer, node **output);

#endif
