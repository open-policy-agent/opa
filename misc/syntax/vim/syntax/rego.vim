" Vim syntax file
" Language: Rego (http://github.com/open-policy-agent/opa)
" Maintainers: Torin Sandall <torinsandall@gmail.com>

if version < 600
    syntax clear
elseif exists("b:current_syntax")
    finish
endif

syn case match

" language keywords
syn keyword regoKeyword package import as not with

" comments
syn match regoComment "#.*$" contains=regoTodo,@Spell
syn keyword regoTodo FIXME XXX TODO contained

" data types
syn keyword regoNull null
syn keyword regoBoolean true false
syn match regoNumber "\<\(0[0-7]*\|0[xx]\x\+\|\d\+\)[ll]\=\>"
syn match regoNumber "\(\<\d\+\.\d*\|\.\d\+\)\([ee][-+]\=\d\+\)\=[ffdd]\="
syn match regoNumber "\<\d\+[ee][-+]\=\d\+[ffdd]\=\>"
syn match regoNumber "\<\d\+\([ee][-+]\=\d\+\)\=[ffdd]\>"
syn region regoString start="\"[^"]" skip="\\\"" end="\"" contains=regoStringEscape
syn match regoStringEscape "\\u[0-9a-fA-F]\{4}" contained
syn match regoStringEscape "\\[nrfvb\\\"]" contained

" rule head
syn match regoRuleName "^\w\+" nextgroup=regoRuleKey,regoRuleValue skipwhite
syn region regoRuleKey start="\[" end="\]" contained skipwhite
syn match regoRuleValue "=\w\+" nextgroup=regoIfThen skipwhite

" operators
syn match regoIfThen ":-"
syn match regoEquality "="
syn match regoInequality "[<>!]"
syn match regoBuiltin "\w\+(" nextgroup=regoBuiltinArgs contains=regoBuiltinArgs
syn region regoBuiltinArgs start="(" end=")" contained contains=regoNumber,regoNull,regoBoolean,regoString

" highlighting
hi link regoKeyword Keyword
hi link regoNull Function
hi link regoBoolean Boolean
hi link regoNumber Number
hi link regoString String
hi link regoRuleName Function
hi link regoRuleKey Normal
hi link regoRuleValue Normal
hi link regoIfThen Keyword
hi link regoEquality Keyword
hi link regoInequality Keyword
hi link regoBuiltin Keyword
hi link regoComment Comment
hi link regoTodo Todo
