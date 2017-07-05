exports.test_data = {
    default_options: [
        { name: "indent_size", value: "4" },
        { name: "indent_char", value: "' '" },
        { name: "preserve_newlines", value: "true" },
        { name: "jslint_happy", value: "false" },
        { name: "keep_array_indentation", value: "false" },
        { name: "brace_style", value: "'collapse'" }
    ],
    groups: [{
        name: "Unicode Support",
        description: "",
        tests: [
            {
              input: "var ' + unicode_char(3232) + '_' + unicode_char(3232) + ' = \"hi\";"
            },
            {
                input: [
                    "var ' + unicode_char(228) + 'x = {",
                    "    ' + unicode_char(228) + 'rgerlich: true",
                    "};"]
            }
        ],
    }, {
        name: "End With Newline",
        description: "",
        matrix: [
            {
                options: [
                    { name: "end_with_newline", value: "true" }
                ],
                eof: '\\n'
            }, {
                options: [
                    { name: "end_with_newline", value: "false" }
                ],
                eof: ''
            }

        ],
        tests: [
            { fragment: true, input: '', output: '{{eof}}' },
            { fragment: true, input: '   return .5', output: '   return .5{{eof}}' },
            { fragment: true, input: '   \n\nreturn .5\n\n\n\n', output: '   return .5{{eof}}' },
            { fragment: true, input: '\n', output: '{{eof}}' }
        ],
    }, {
        name: "Comma-first option",
        description: "Put commas at the start of lines instead of the end",
        matrix: [
        {
            options: [
                { name: "comma_first", value: "true" }
            ],
            c0: '\\n, ',
            c1: '\\n    , ',
            c2: '\\n        , ',
            c3: '\\n            , '
        }, {
            options: [
                { name: "comma_first", value: "false" }
            ],
            c0: ',\\n',
            c1: ',\\n    ',
            c2: ',\\n        ',
            c3: ',\\n            '
        }
        ],
        tests: [
            { input: '{a:1, b:2}', output: "{\n    a: 1{{c1}}b: 2\n}" },
            { input: 'var a=1, b=c[d], e=6;', output: 'var a = 1{{c1}}b = c[d]{{c1}}e = 6;' },
            { input: "for(var a=1,b=2,c=3;d<3;d++)\ne", output: "for (var a = 1, b = 2, c = 3; d < 3; d++)\n    e" },
            { input: "for(var a=1,b=2,\nc=3;d<3;d++)\ne", output: "for (var a = 1, b = 2{{c2}}c = 3; d < 3; d++)\n    e" },
            { input: 'function foo() {\n    return [\n        "one"{{c2}}"two"\n    ];\n}' },
            { input: 'a=[[1,2],[4,5],[7,8]]', output: "a = [\n    [1, 2]{{c1}}[4, 5]{{c1}}[7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],[7,8],]', output: "a = [\n    [1, 2]{{c1}}[4, 5]{{c1}}[7, 8]{{c0}}]" },
            { input: 'a=[[1,2],[4,5],function(){},[7,8]]',
            output: "a = [\n    [1, 2]{{c1}}[4, 5]{{c1}}function() {}{{c1}}[7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],function(){},function(){},[7,8]]',
            output: "a = [\n    [1, 2]{{c1}}[4, 5]{{c1}}function() {}{{c1}}function() {}{{c1}}[7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],function(){},[7,8]]',
            output: "a = [\n    [1, 2]{{c1}}[4, 5]{{c1}}function() {}{{c1}}[7, 8]\n]" },
            { input: 'a=[b,c,function(){},function(){},d]',
            output: "a = [b, c, function() {}, function() {}, d]" },
            { input: 'a=[b,c,\nfunction(){},function(){},d]',
            output: "a = [b, c{{c1}}function() {}{{c1}}function() {}{{c1}}d\n]" },
            { input: 'a=[a[1],b[4],c[d[7]]]', output: "a = [a[1], b[4], c[d[7]]]" },
            { input: '[1,2,[3,4,[5,6],7],8]', output: "[1, 2, [3, 4, [5, 6], 7], 8]" },

            { input: '[[["1","2"],["3","4"]],[["5","6","7"],["8","9","0"]],[["1","2","3"],["4","5","6","7"],["8","9","0"]]]',
            output: '[\n    [\n        ["1", "2"]{{c2}}["3", "4"]\n    ]{{c1}}[\n        ["5", "6", "7"]{{c2}}["8", "9", "0"]\n    ]{{c1}}[\n        ["1", "2", "3"]{{c2}}["4", "5", "6", "7"]{{c2}}["8", "9", "0"]\n    ]\n]' },

        ],
    }, {
        name: "New Test Suite"
    },
    {
        name: "Async / await tests",
        description: "ES7 async / await tests",
        tests: [
            { input: "async function foo() {}" },
            { input: "let w = async function foo() {}" },
            { input: "async function foo() {}\nvar x = await foo();"},
            {
                comment: "async function as an input to another function",
                input: "wrapper(async function foo() {})"},
            {
                comment: "await on inline anonymous function. should have a space after await",
                input_: "async function() {\n    var w = await(async function() {\n        return await foo();\n    })();\n}",
                output: "async function() {\n    var w = await (async function() {\n        return await foo();\n    })();\n}"
            },
            {
                comment: "ensure that this doesn't break anyone with the async library",
                input: "async.map(function(t) {})"
            }
        ]
    },
    {
        name: "e4x - Test that e4x literals passed through when e4x-option is enabled",
        description: "",
        options: [
            { name: 'e4x', value: true }
        ],
        tests: [
            { input: 'xml=<a b="c"><d/><e>\n foo</e>x</a>;', output: 'xml = <a b="c"><d/><e>\n foo</e>x</a>;' },
            { unchanged: '<a b=\\\'This is a quoted "c".\\\'/>' },
            { unchanged: '<a b="This is a quoted \\\'c\\\'."/>' },
            { unchanged: '<a b="A quote \\\' inside string."/>' },
            { unchanged: '<a b=\\\'A quote " inside string.\\\'/>' },
            { unchanged: '<a b=\\\'Some """ quotes ""  inside string.\\\'/>' },

            {
                comment: 'Handles inline expressions',
                input: 'xml=<{a} b="c"><d/><e v={z}>\n foo</e>x</{a}>;',
                output: 'xml = <{a} b="c"><d/><e v={z}>\n foo</e>x</{a}>;' },
            {
                input: 'xml=<{a} b="c">\n    <e v={z}>\n foo</e>x</{a}>;',
                output: 'xml = <{a} b="c">\n    <e v={z}>\n foo</e>x</{a}>;' },
            {
                comment: 'xml literals with special characters in elem names - see http://www.w3.org/TR/REC-xml/#NT-NameChar',
                unchanged: 'xml = <_:.valid.xml- _:.valid.xml-="123"/>;'
            },

            {
                comment: 'Handles CDATA',
                input: 'xml=<![CDATA[ b="c"><d/><e v={z}>\n foo</e>x/]]>;',
                output: 'xml = <![CDATA[ b="c"><d/><e v={z}>\n foo</e>x/]]>;' },
            { input: 'xml=<![CDATA[]]>;', output: 'xml = <![CDATA[]]>;' },
            { input: 'xml=<a b="c"><![CDATA[d/></a></{}]]></a>;', output: 'xml = <a b="c"><![CDATA[d/></a></{}]]></a>;' },

            {
                comment: 'JSX - working jsx from http://prettydiff.com/unit_tests/beautification_javascript_jsx.txt',
                unchanged:
                [
                    'var ListItem = React.createClass({',
                    '    render: function() {',
                    '        return (',
                    '            <li className="ListItem">',
                    '                <a href={ "/items/" + this.props.item.id }>',
                    '                    this.props.item.name',
                    '                </a>',
                    '            </li>',
                    '        );',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged:
                [
                    'var List = React.createClass({',
                    '    renderList: function() {',
                    '        return this.props.items.map(function(item) {',
                    '            return <ListItem item={item} key={item.id} />;',
                    '        });',
                    '    },',
                    '',
                    '    render: function() {',
                    '        return <ul className="List">',
                    '                this.renderList()',
                    '            </ul>',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged:
                [
                    'var Mist = React.createClass({',
                    '    renderList: function() {',
                    '        return this.props.items.map(function(item) {',
                    '            return <ListItem item={return <tag>{item}</tag>} key={item.id} />;',
                    '        });',
                    '    }',
                    '});',
                ]
            },
            {
                unchanged:
                [
                    '// JSX',
                    'var box = <Box>',
                    '    {shouldShowAnswer(user) ?',
                    '        <Answer value={false}>no</Answer> : <Box.Comment>',
                    '        Text Content',
                    '        </Box.Comment>}',
                    '    </Box>;',
                    'var a = function() {',
                    '    return <tsdf>asdf</tsdf>;',
                    '};',
                    '',
                    'var HelloMessage = React.createClass({',
                    '    render: function() {',
                    '        return <div>Hello {this.props.name}</div>;',
                    '    }',
                    '});',
                    'React.render(<HelloMessage name="John" />, mountNode);',
                ]
            },
            {
                unchanged:
                [
                    'var Timer = React.createClass({',
                    '    getInitialState: function() {',
                    '        return {',
                    '            secondsElapsed: 0',
                    '        };',
                    '    },',
                    '    tick: function() {',
                    '        this.setState({',
                    '            secondsElapsed: this.state.secondsElapsed + 1',
                    '        });',
                    '    },',
                    '    componentDidMount: function() {',
                    '        this.interval = setInterval(this.tick, 1000);',
                    '    },',
                    '    componentWillUnmount: function() {',
                    '        clearInterval(this.interval);',
                    '    },',
                    '    render: function() {',
                    '        return (',
                    '            <div>Seconds Elapsed: {this.state.secondsElapsed}</div>',
                    '        );',
                    '    }',
                    '});',
                    'React.render(<Timer />, mountNode);'
                ]
            },
            {
                unchanged:
                [
                    'var TodoList = React.createClass({',
                    '    render: function() {',
                    '        var createItem = function(itemText) {',
                    '            return <li>{itemText}</li>;',
                    '        };',
                    '        return <ul>{this.props.items.map(createItem)}</ul>;',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged:
                [
                    'var TodoApp = React.createClass({',
                    '    getInitialState: function() {',
                    '        return {',
                    '            items: [],',
                    '            text: \\\'\\\'',
                    '        };',
                    '    },',
                    '    onChange: function(e) {',
                    '        this.setState({',
                    '            text: e.target.value',
                    '        });',
                    '    },',
                    '    handleSubmit: function(e) {',
                    '        e.preventDefault();',
                    '        var nextItems = this.state.items.concat([this.state.text]);',
                    '        var nextText = \\\'\\\';',
                    '        this.setState({',
                    '            items: nextItems,',
                    '            text: nextText',
                    '        });',
                    '    },',
                    '    render: function() {',
                    '        return (',
                    '            <div>',
                    '                <h3>TODO</h3>',
                    '                <TodoList items={this.state.items} />',
                    '                <form onSubmit={this.handleSubmit}>',
                    '                    <input onChange={this.onChange} value={this.state.text} />',
                    '                    <button>{\\\'Add #\\\' + (this.state.items.length + 1)}</button>',
                    '                </form>',
                    '            </div>',
                    '        );',
                    '    }',
                    '});',
                    'React.render(<TodoApp />, mountNode);'
                ]
            },
            {
                input:
                [
                    'var converter = new Showdown.converter();',
                    'var MarkdownEditor = React.createClass({',
                    '    getInitialState: function() {',
                    '        return {value: \\\'Type some *markdown* here!\\\'};',
                    '    },',
                    '    handleChange: function() {',
                    '        this.setState({value: this.refs.textarea.getDOMNode().value});',
                    '    },',
                    '    render: function() {',
                    '        return (',
                    '            <div className="MarkdownEditor">',
                    '                <h3>Input</h3>',
                    '                <textarea',
                    '                    onChange={this.handleChange}',
                    '                    ref="textarea"',
                    '                    defaultValue={this.state.value} />',
                    '                <h3>Output</h3>',
                    '            <div',
                    '                className="content"',
                    '                dangerouslySetInnerHTML={{',
                    '                        __html: converter.makeHtml(this.state.value)',
                    '                    }}',
                    '                />',
                    '            </div>',
                    '        );',
                    '    }',
                    '});',
                    'React.render(<MarkdownEditor />, mountNode);'

                ],
                output:
                [
                    'var converter = new Showdown.converter();',
                    'var MarkdownEditor = React.createClass({',
                    '    getInitialState: function() {',
                    '        return {',
                    '            value: \\\'Type some *markdown* here!\\\'',
                    '        };',
                    '    },',
                    '    handleChange: function() {',
                    '        this.setState({',
                    '            value: this.refs.textarea.getDOMNode().value',
                    '        });',
                    '    },',
                    '    render: function() {',
                    '        return (',
                    '            <div className="MarkdownEditor">',
                    '                <h3>Input</h3>',
                    '                <textarea',
                    '                    onChange={this.handleChange}',
                    '                    ref="textarea"',
                    '                    defaultValue={this.state.value} />',
                    '                <h3>Output</h3>',
                    '            <div',
                    '                className="content"',
                    '                dangerouslySetInnerHTML={{',
                    '                        __html: converter.makeHtml(this.state.value)',
                    '                    }}',
                    '                />',
                    '            </div>',
                    '        );',
                    '    }',
                    '});',
                    'React.render(<MarkdownEditor />, mountNode);'
                ]
            },
            {
                comment: 'JSX - Not quite correct jsx formatting that still works',
                input:
                [
                    'var content = (',
                    '        <Nav>',
                    '            {/* child comment, put {} around */}',
                    '            <Person',
                    '                /* multi',
                    '         line',
                    '         comment */',
                    '         //attr="test"',
                    '                name={window.isLoggedIn ? window.name : \\\'\\\'} // end of line comment',
                    '            />',
                    '        </Nav>',
                    '    );',
                    'var qwer = <DropDown> A dropdown list <Menu> <MenuItem>Do Something</MenuItem> <MenuItem>Do Something Fun!</MenuItem> <MenuItem>Do Something Else</MenuItem> </Menu> </DropDown>;',
                    'render(dropdown);',
                ],
                output:
                [
                    'var content = (',
                    '    <Nav>',
                    '            {/* child comment, put {} around */}',
                    '            <Person',
                    '                /* multi',
                    '         line',
                    '         comment */',
                    '         //attr="test"',
                    '                name={window.isLoggedIn ? window.name : \\\'\\\'} // end of line comment',
                    '            />',
                    '        </Nav>',
                    ');',
                    'var qwer = <DropDown> A dropdown list <Menu> <MenuItem>Do Something</MenuItem> <MenuItem>Do Something Fun!</MenuItem> <MenuItem>Do Something Else</MenuItem> </Menu> </DropDown>;',
                    'render(dropdown);',
                ]
            },
            {
                comment: [
                    "Handles messed up tags, as long as it isn't the same name",
                    "as the root tag. Also handles tags of same name as root tag",
                    "as long as nesting matches."
                ],
                input_: 'xml=<a x="jn"><c></b></f><a><d jnj="jnn"><f></a ></nj></a>;',
                output: 'xml = <a x="jn"><c></b></f><a><d jnj="jnn"><f></a ></nj></a>;' },

            {
                comment: [
                    "If xml is not terminated, the remainder of the file is treated",
                    "as part of the xml-literal (passed through unaltered)"
                ],
                fragment: true,
                input_: 'xml=<a></b>\nc<b;',
                output: 'xml = <a></b>\nc<b;' },
            {
                comment: 'Issue #646 = whitespace is allowed in attribute declarations',
                unchanged: [
                    'let a = React.createClass({',
                    '    render() {',
                    '        return (',
                    '            <p className=\\\'a\\\'>',
                    '                <span>c</span>',
                    '            </p>',
                    '        );',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged: [
                    'let a = React.createClass({',
                    '    render() {',
                    '        return (',
                    '            <p className = \\\'b\\\'>',
                    '                <span>c</span>',
                    '            </p>',
                    '        );',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged: [
                    'let a = React.createClass({',
                    '    render() {',
                    '        return (',
                    '            <p className = "c">',
                    '                <span>c</span>',
                    '            </p>',
                    '        );',
                    '    }',
                    '});'
                ]
            },
            {
                unchanged: [
                    'let a = React.createClass({',
                    '    render() {',
                    '        return (',
                    '            <{e}  className = {d}>',
                    '                <span>c</span>',
                    '            </{e}>',
                    '        );',
                    '    }',
                    '});'
                ]
            }
        ]
    },
    {
        name: "e4x disabled",
        description: "",
        options: [
            { name: 'e4x', value: false }
        ],
        tests: [
            {
                input_: 'xml=<a b="c"><d/><e>\n foo</e>x</a>;',
                output: 'xml = < a b = "c" > < d / > < e >\n    foo < /e>x</a > ;'
            }
        ]
    },
    {
        name: "Multiple braces",
        description: "",
        template: "^^^ $$$",
        options: [],
        tests: [
            { input: '{{}/z/}', output: '{\n    {}\n    /z/\n}' }
        ]
    },
    {
        name: "Beautify preserve formatting",
        description: "Allow beautifier to preserve sections",
        tests: [
            { unchanged: "/* beautify preserve:start */\n/* beautify preserve:end */" },
            { unchanged: "/* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */" },
            { unchanged: "var a = 1;\n/* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */" },
            { unchanged: "/* beautify preserve:start */     {asdklgh;y;;{}dd2d}/* beautify preserve:end */" },
            {
              input_: "var a =  1;\n/* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */",
              output: "var a = 1;\n/* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */"
            },
            {
              input_: "var a = 1;\n /* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */",
              output: "var a = 1;\n/* beautify preserve:start */\n   var a = 1;\n/* beautify preserve:end */"
            },
            {
                unchanged: [
                    'var a = {',
                    '    /* beautify preserve:start */',
                    '    one   :  1',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '    /* beautify preserve:end */',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/* beautify preserve:start */',
                    '    one   :  1,',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '/* beautify preserve:end */',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify preserve:start */',
                    '    one   :  1,',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '/* beautify preserve:end */',
                    '};'
                ]
            },
            {
                comment: 'one space before and after required, only single spaces inside.',
                input: [
                    'var a = {',
                    '/*  beautify preserve:start  */',
                    '    one   :  1,',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /*  beautify preserve:start  */',
                    '    one: 1,',
                    '    two: 2,',
                    '    three: 3,',
                    '    ten: 10',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/*beautify preserve:start*/',
                    '    one   :  1,',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /*beautify preserve:start*/',
                    '    one: 1,',
                    '    two: 2,',
                    '    three: 3,',
                    '    ten: 10',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/*beautify  preserve:start*/',
                    '    one   :  1,',
                    '    two   :  2,',
                    '    three :  3,',
                    '    ten   : 10',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /*beautify  preserve:start*/',
                    '    one: 1,',
                    '    two: 2,',
                    '    three: 3,',
                    '    ten: 10',
                    '};'
                ]
            },

            { comment: 'Directive: ignore',
              unchanged: "/* beautify ignore:start */\n/* beautify ignore:end */" },
            { unchanged: "/* beautify ignore:start */\n   var a,,,{ 1;\n/* beautify ignore:end */" },
            { unchanged: "var a = 1;\n/* beautify ignore:start */\n   var a = 1;\n/* beautify ignore:end */" },
            { unchanged: "/* beautify ignore:start */     {asdklgh;y;+++;dd2d}/* beautify ignore:end */" },
            {
              input_: "var a =  1;\n/* beautify ignore:start */\n   var a,,,{ 1;\n/* beautify ignore:end */",
              output: "var a = 1;\n/* beautify ignore:start */\n   var a,,,{ 1;\n/* beautify ignore:end */"
            },
            {
              input_: "var a = 1;\n /* beautify ignore:start */\n   var a,,,{ 1;\n/* beautify ignore:end */",
              output: "var a = 1;\n/* beautify ignore:start */\n   var a,,,{ 1;\n/* beautify ignore:end */"
            },
            {
                unchanged: [
                    'var a = {',
                    '    /* beautify ignore:start */',
                    '    one   :  1',
                    '    two   :  2,',
                    '    three :  {',
                    '    ten   : 10',
                    '    /* beautify ignore:end */',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/* beautify ignore:start */',
                    '    one   :  1',
                    '    two   :  2,',
                    '    three :  {',
                    '    ten   : 10',
                    '/* beautify ignore:end */',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify ignore:start */',
                    '    one   :  1',
                    '    two   :  2,',
                    '    three :  {',
                    '    ten   : 10',
                    '/* beautify ignore:end */',
                    '};'
                ]
            },
            {
                comment: 'Directives - multiple and interacting',
                input: [
                    'var a = {',
                    '/* beautify preserve:start */',
                    '/* beautify preserve:start */',
                    '    one   :  1,',
                    '  /* beautify preserve:end */',
                    '    two   :  2,',
                    '    three :  3,',
                    '/* beautify preserve:start */',
                    '    ten   : 10',
                    '/* beautify preserve:end */',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify preserve:start */',
                    '/* beautify preserve:start */',
                    '    one   :  1,',
                    '  /* beautify preserve:end */',
                    '    two: 2,',
                    '    three: 3,',
                    '    /* beautify preserve:start */',
                    '    ten   : 10',
                    '/* beautify preserve:end */',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/* beautify ignore:start */',
                    '    one   :  1',
                    ' /* beautify ignore:end */',
                    '    two   :  2,',
                    '/* beautify ignore:start */',
                    '    three :  {',
                    '    ten   : 10',
                    '/* beautify ignore:end */',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify ignore:start */',
                    '    one   :  1',
                    ' /* beautify ignore:end */',
                    '    two: 2,',
                    '    /* beautify ignore:start */',
                    '    three :  {',
                    '    ten   : 10',
                    '/* beautify ignore:end */',
                    '};'
                ]
            },
            {
                comment: 'Starts can occur together, ignore:end must occur alone.',
                input: [
                    'var a = {',
                    '/* beautify ignore:start */',
                    '    one   :  1',
                    '    NOTE: ignore end block does not support starting other directives',
                    '    This does not match the ending the ignore...',
                    ' /* beautify ignore:end preserve:start */',
                    '    two   :  2,',
                    '/* beautify ignore:start */',
                    '    three :  {',
                    '    ten   : 10',
                    '    ==The next comment ends the starting ignore==',
                    '/* beautify ignore:end */',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify ignore:start */',
                    '    one   :  1',
                    '    NOTE: ignore end block does not support starting other directives',
                    '    This does not match the ending the ignore...',
                    ' /* beautify ignore:end preserve:start */',
                    '    two   :  2,',
                    '/* beautify ignore:start */',
                    '    three :  {',
                    '    ten   : 10',
                    '    ==The next comment ends the starting ignore==',
                    '/* beautify ignore:end */',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/* beautify ignore:start preserve:start */',
                    '    one   :  {',
                    ' /* beautify ignore:end */',
                    '    two   :  2,',
                    '  /* beautify ignore:start */',
                    '    three :  {',
                    '/* beautify ignore:end */',
                    '    ten   : 10',
                    '   // This is all preserved',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify ignore:start preserve:start */',
                    '    one   :  {',
                    ' /* beautify ignore:end */',
                    '    two   :  2,',
                    '  /* beautify ignore:start */',
                    '    three :  {',
                    '/* beautify ignore:end */',
                    '    ten   : 10',
                    '   // This is all preserved',
                    '};'
                ]
            },
            {
                input: [
                    'var a = {',
                    '/* beautify ignore:start preserve:start */',
                    '    one   :  {',
                    ' /* beautify ignore:end */',
                    '    two   :  2,',
                    '  /* beautify ignore:start */',
                    '    three :  {',
                    '/* beautify ignore:end */',
                    '    ten   : 10,',
                    '/* beautify preserve:end */',
                    '     eleven: 11',
                    '};'
                ],
                output: [
                    'var a = {',
                    '    /* beautify ignore:start preserve:start */',
                    '    one   :  {',
                    ' /* beautify ignore:end */',
                    '    two   :  2,',
                    '  /* beautify ignore:start */',
                    '    three :  {',
                    '/* beautify ignore:end */',
                    '    ten   : 10,',
                    '/* beautify preserve:end */',
                    '    eleven: 11',
                    '};'
                ]
            },
        ]
    },
    {
        name: "Template Formatting",
        description: "Php (<?php ... ?>) and underscore.js templating treated as strings.",
        options: [],
        tests: [
            { unchanged: '<?=$view["name"]; ?>' },
            { unchanged: 'a = <?= external() ?>;' },
            { unchanged:
                [
                    '<?php',
                    'for($i = 1; $i <= 100; $i++;) {',
                    '    #count to 100!',
                    '    echo($i . "</br>");',
                    '}',
                    '?>'
                ]
            },
            { unchanged: 'a = <%= external() %>;' }
        ]
    },
    {
        name: "jslint and space after anon function",
        description: "jslint_happy and space_after_anon_function tests",
        matrix: [
            {
                options: [
                    { name: "jslint_happy", value: "true" },
                    { name: "space_after_anon_function", value: "true" }
                ],
                f: ' ',
                c: ''
            }, {
                options: [
                    { name: "jslint_happy", value: "true" },
                    { name: "space_after_anon_function", value: "false" }
                ],
                f: ' ',
                c: ''
            }, {
                options: [
                    { name: "jslint_happy", value: "false" },
                    { name: "space_after_anon_function", value: "true" }
                ],
                f: ' ',
                c: '    '
            }, {
                options: [
                    { name: "jslint_happy", value: "false" },
                    { name: "space_after_anon_function", value: "false" }
                ],
                f: '',
                c: '    '
            }


        ],
        tests: [
            { input_: 'a=typeof(x)',
                output: 'a = typeof{{f}}(x)' },
            { input_: 'x();\n\nfunction(){}',
                output: 'x();\n\nfunction{{f}}() {}' },
            { input_: 'function () {\n    var a, b, c, d, e = [],\n        f;\n}',
                output: 'function{{f}}() {\n    var a, b, c, d, e = [],\n        f;\n}' },

            { input_: 'switch(x) {case 0: case 1: a(); break; default: break}',
                output: 'switch (x) {\n{{c}}case 0:\n{{c}}case 1:\n{{c}}    a();\n{{c}}    break;\n{{c}}default:\n{{c}}    break\n}' },
            { input: 'switch(x){case -1:break;case !y:break;}',
                output: 'switch (x) {\n{{c}}case -1:\n{{c}}    break;\n{{c}}case !y:\n{{c}}    break;\n}' },
            { comment: 'typical greasemonkey start',
                fragment: true,
                unchanged: '// comment 2\n(function{{f}}()'
            },

            {
                input_: 'var a2, b2, c2, d2 = 0, c = function() {}, d = \\\'\\\';',
                output: 'var a2, b2, c2, d2 = 0,\n    c = function{{f}}() {},\n    d = \\\'\\\';'
            },
            {
                input_: 'var a2, b2, c2, d2 = 0, c = function() {},\nd = \\\'\\\';',
                output: 'var a2, b2, c2, d2 = 0,\n    c = function{{f}}() {},\n    d = \\\'\\\';'
            },
            {
                input_: 'var o2=$.extend(a);function(){alert(x);}',
                output: 'var o2 = $.extend(a);\n\nfunction{{f}}() {\n    alert(x);\n}'
            },
            { input: 'function*() {\n    yield 1;\n}', output: 'function*{{f}}() {\n    yield 1;\n}'},
            { unchanged: 'function* x() {\n    yield 1;\n}' },
        ]
    }, {
        name: "Regression tests",
        description: "Ensure specific bugs do not recur",
        options: [],
        tests: [
            {
                comment: "Issue 241",
                unchanged: [
                    'obj',
                    '    .last({',
                    '        foo: 1,',
                    '        bar: 2',
                    '    });',
                    'var test = 1;' ]
            },
            {
                unchanged: [
                    'obj',
                    '    .last(a, function() {',
                    '        var test;',
                    '    });',
                    'var test = 1;' ]
            },
            {
                unchanged: [
                    'obj.first()',
                    '    .second()',
                    '    .last(function(err, response) {',
                    '        console.log(err);',
                    '    });' ]
            },
            {
                comment: "Issue 268 and 275",
                unchanged: [
                    'obj.last(a, function() {',
                    '    var test;',
                    '});',
                    'var test = 1;' ]
            },
            {
                unchanged: [
                    'obj.last(a,',
                    '    function() {',
                    '        var test;',
                    '    });',
                    'var test = 1;' ]
            },
            {
                input: '(function() {if (!window.FOO) window.FOO || (window.FOO = function() {var b = {bar: "zort"};});})();',
                output: [
                    '(function() {',
                    '    if (!window.FOO) window.FOO || (window.FOO = function() {',
                    '        var b = {',
                    '            bar: "zort"',
                    '        };',
                    '    });',
                    '})();' ]
            },
            {
                comment: "Issue 281",
                unchanged: [
                    'define(["dojo/_base/declare", "my/Employee", "dijit/form/Button",',
                    '    "dojo/_base/lang", "dojo/Deferred"',
                    '], function(declare, Employee, Button, lang, Deferred) {',
                    '    return declare(Employee, {',
                    '        constructor: function() {',
                    '            new Button({',
                    '                onClick: lang.hitch(this, function() {',
                    '                    new Deferred().then(lang.hitch(this, function() {',
                    '                        this.salary * 0.25;',
                    '                    }));',
                    '                })',
                    '            });',
                    '        }',
                    '    });',
                    '});' ]
            },
            {
                unchanged: [
                    'define(["dojo/_base/declare", "my/Employee", "dijit/form/Button",',
                    '        "dojo/_base/lang", "dojo/Deferred"',
                    '    ],',
                    '    function(declare, Employee, Button, lang, Deferred) {',
                    '        return declare(Employee, {',
                    '            constructor: function() {',
                    '                new Button({',
                    '                    onClick: lang.hitch(this, function() {',
                    '                        new Deferred().then(lang.hitch(this, function() {',
                    '                            this.salary * 0.25;',
                    '                        }));',
                    '                    })',
                    '                });',
                    '            }',
                    '        });',
                    '    });' ]
            },
            {
                comment: "Issue 459",
                unchanged: [
                    '(function() {',
                    '    return {',
                    '        foo: function() {',
                    '            return "bar";',
                    '        },',
                    '        bar: ["bar"]',
                    '    };',
                    '}());' ]
            },
            {
                comment: "Issue 505 - strings should end at newline unless continued by backslash",
                unchanged: [
                    'var name = "a;',
                    'name = "b";' ]
            },
            {
                unchanged: [
                    'var name = "a;\\\\',
                    '    name = b";' ]
            },
            {
                comment: "Issue 514 - some operators require spaces to distinguish them",
                unchanged: 'var c = "_ACTION_TO_NATIVEAPI_" + ++g++ + +new Date;'
            },
            {
                unchanged: 'var c = "_ACTION_TO_NATIVEAPI_" - --g-- - -new Date;'
            },
            {
                comment: "Issue 440 - reserved words can be used as object property names",
                unchanged: [
                    'a = {',
                    '    function: {},',
                    '    "function": {},',
                    '    throw: {},',
                    '    "throw": {},',
                    '    var: {},',
                    '    "var": {},',
                    '    set: {},',
                    '    "set": {},',
                    '    get: {},',
                    '    "get": {},',
                    '    if: {},',
                    '    "if": {},',
                    '    then: {},',
                    '    "then": {},',
                    '    else: {},',
                    '    "else": {},',
                    '    yay: {}',
                    '};' ]
            },
            {
                comment: "Issue 331 - if-else with braces edge case",
                input: 'if(x){a();}else{b();}if(y){c();}',
                output: [
                    'if (x) {',
                    '    a();',
                    '} else {',
                    '    b();',
                    '}',
                    'if (y) {',
                    '    c();',
                    '}' ]
            },
            {
                comment: "Issue 485 - ensure function declarations behave the same in arrays as elsewhere",
                unchanged: [
                    'var v = ["a",',
                    '    function() {',
                    '        return;',
                    '    }, {',
                    '        id: 1',
                    '    }',
                    '];' ]
            },
            {
                unchanged: [
                    'var v = ["a", function() {',
                    '    return;',
                    '}, {',
                    '    id: 1',
                    '}];' ]
            },
            {
                comment: "Issue 382 - initial totally cursory support for es6 module export",
                unchanged: [
                    'module "Even" {',
                    '    import odd from "Odd";',
                    '    export function sum(x, y) {',
                    '        return x + y;',
                    '    }',
                    '    export var pi = 3.141593;',
                    '    export default moduleName;',
                    '}' ]
            },
            {
                unchanged: [
                    'module "Even" {',
                    '    export default function div(x, y) {}',
                    '}' ]
            },
            {
                comment: "Issue 508",
                unchanged: 'set["name"]'
            },
            {
                unchanged: 'get["name"]'
            },
            {
                fragmeent: true,
                unchanged: [
                    'a = {',
                    '    set b(x) {},',
                    '    c: 1,',
                    '    d: function() {}',
                    '};' ]
            },
            {
                fragmeent: true,
                unchanged: [
                    'a = {',
                    '    get b() {',
                    '        retun 0;',
                    '    },',
                    '    c: 1,',
                    '    d: function() {}',
                    '};' ]
            },
            {
                comment: "Issue 298 - do not under indent if/while/for condtionals experesions",
                unchanged: [
                    '\\\'use strict\\\';',
                    'if ([].some(function() {',
                    '        return false;',
                    '    })) {',
                    '    console.log("hello");',
                    '}' ]
            },
            {
                comment: "Issue 298 - do not under indent if/while/for condtionals experesions",
                unchanged: [
                    '\\\'use strict\\\';',
                    'if ([].some(function() {',
                    '        return false;',
                    '    })) {',
                    '    console.log("hello");',
                    '}' ]
            },
            {
                comment: "Issue 552 - Typescript?  Okay... we didn't break it before, so try not to break it now.",
                unchanged: [
                    'class Test {',
                    '    blah: string[];',
                    '    foo(): number {',
                    '        return 0;',
                    '    }',
                    '    bar(): number {',
                    '        return 0;',
                    '    }',
                    '}' ]
            },
            {
                unchanged: [
                    'interface Test {',
                    '    blah: string[];',
                    '    foo(): number {',
                    '        return 0;',
                    '    }',
                    '    bar(): number {',
                    '        return 0;',
                    '    }',
                    '}' ]
            },
            {
                comment: "Issue 583 - Functions with comments after them should still indent correctly.",
                unchanged: [
                    'function exit(code) {',
                    '    setTimeout(function() {',
                    '        phantom.exit(code);',
                    '    }, 0);',
                    '    phantom.onError = function() {};',
                    '}',
                    '// Comment' ]
            },

        ]
    },

        // =======================================================
        // New tests groups should be added above this line.
        // Everything below is a work in progress - converting
        // old test to generated form.
        // =======================================================
    {
        name: "Old tests",
        description: "Largely unorganized pile of tests",
        options: [],
        tests: [
            { unchanged: '' },
            { fragment: true, unchanged: '   return .5'},
            { fragment: true, unchanged: '   return .5;\n   a();' },
            { fragment: true, unchanged: '    return .5;\n    a();' },
            { fragment: true, unchanged: '     return .5;\n     a();' },
            { fragment: true, unchanged: '   < div'},
            { input: 'a        =          1', output: 'a = 1' },
            { input: 'a=1', output: 'a = 1' },
            { unchanged: '(3) / 2' },
            { input: '["a", "b"].join("")' },
            { unchanged: 'a();\n\nb();' },
            { input: 'var a = 1 var b = 2', output: 'var a = 1\nvar b = 2' },
            { input: 'var a=1, b=c[d], e=6;', output: 'var a = 1,\n    b = c[d],\n    e = 6;' },
            { unchanged: 'var a,\n    b,\n    c;' },
            { input: 'let a = 1 let b = 2', output: 'let a = 1\nlet b = 2' },
            { input: 'let a=1, b=c[d], e=6;', output: 'let a = 1,\n    b = c[d],\n    e = 6;' },
            { unchanged: 'let a,\n    b,\n    c;' },
            { input: 'const a = 1 const b = 2', output: 'const a = 1\nconst b = 2' },
            { input: 'const a=1, b=c[d], e=6;', output: 'const a = 1,\n    b = c[d],\n    e = 6;' },
            { unchanged: 'const a,\n    b,\n    c;' },
            { unchanged: 'a = " 12345 "' },
            { unchanged: "a = \\' 12345 \\'" },
            { unchanged: 'if (a == 1) b = 2;' },
            { input: 'if(1){2}else{3}', output: 'if (1) {\n    2\n} else {\n    3\n}' },
            { input: 'if(1||2);', output: 'if (1 || 2);' },
            { input: '(a==1)||(b==2)', output: '(a == 1) || (b == 2)' },
            { input: 'var a = 1 if (2) 3;', output: 'var a = 1\nif (2) 3;' },
            { unchanged: 'a = a + 1' },
            { unchanged: 'a = a == 1' },
            { input: '/12345[^678]*9+/.match(a)' },
            { unchanged: 'a /= 5' },
            { unchanged: 'a = 0.5 * 3' },
            { unchanged: 'a *= 10.55' },
            { unchanged: 'a < .5' },
            { unchanged: 'a <= .5' },
            { input: 'a<.5', output: 'a < .5' },
            { input: 'a<=.5', output: 'a <= .5' },
            { unchanged: 'a = 0xff;' },
            { input: 'a=0xff+4', output: 'a = 0xff + 4' },
            { unchanged: 'a = [1, 2, 3, 4]' },
            { input: 'F*(g/=f)*g+b', output: 'F * (g /= f) * g + b' },
            { input: 'a.b({c:d})', output: 'a.b({\n    c: d\n})' },
            { input: 'a.b\n(\n{\nc:\nd\n}\n)', output: 'a.b({\n    c: d\n})' },
            { input: 'a.b({c:"d"})', output: 'a.b({\n    c: "d"\n})' },
            { input: 'a.b\n(\n{\nc:\n"d"\n}\n)', output: 'a.b({\n    c: "d"\n})' },
            { input: 'a=!b', output: 'a = !b' },
            { input: 'a=!!b', output: 'a = !!b' },
            { input: 'a?b:c', output: 'a ? b : c' },
            { input: 'a?1:2', output: 'a ? 1 : 2' },
            { input: 'a?(b):c', output: 'a ? (b) : c' },
            { input: 'x={a:1,b:w=="foo"?x:y,c:z}', output: 'x = {\n    a: 1,\n    b: w == "foo" ? x : y,\n    c: z\n}' },
            { input: 'x=a?b?c?d:e:f:g;', output: 'x = a ? b ? c ? d : e : f : g;' },
            { input: 'x=a?b?c?d:{e1:1,e2:2}:f:g;', output: 'x = a ? b ? c ? d : {\n    e1: 1,\n    e2: 2\n} : f : g;' },
            { input: 'function void(void) {}' },
            { input: 'if(!a)foo();', output: 'if (!a) foo();' },
            { input: 'a=~a', output: 'a = ~a' },
            { input: 'a;/*comment*/b;', output: "a; /*comment*/\nb;" },
            { input: 'a;/* comment */b;', output: "a; /* comment */\nb;" },
            { fragment: true, input: 'a;/*\ncomment\n*/b;', output: "a;\n/*\ncomment\n*/\nb;", comment: "simple comments don't get touched at all"  },
            { input: 'a;/**\n* javadoc\n*/b;', output: "a;\n/**\n * javadoc\n */\nb;" },
            { fragment: true, input: 'a;/**\n\nno javadoc\n*/b;', output: "a;\n/**\n\nno javadoc\n*/\nb;" },
            { input: 'a;/*\n* javadoc\n*/b;', output: "a;\n/*\n * javadoc\n */\nb;", comment: 'comment blocks detected and reindented even w/o javadoc starter' },
            { input: 'if(a)break;', output: "if (a) break;" },
            { input: 'if(a){break}', output: "if (a) {\n    break\n}" },
            { input: 'if((a))foo();', output: 'if ((a)) foo();' },
            { input: 'for(var i=0;;) a', output: 'for (var i = 0;;) a' },
            { input: 'for(var i=0;;)\na', output: 'for (var i = 0;;)\n    a' },
            { unchanged: 'a++;' },
            { input: 'for(;;i++)a()', output: 'for (;; i++) a()' },
            { input: 'for(;;i++)\na()', output: 'for (;; i++)\n    a()' },
            { input: 'for(;;++i)a', output: 'for (;; ++i) a' },
            { input: 'return(1)', output: 'return (1)' },
            { input: 'try{a();}catch(b){c();}finally{d();}', output: "try {\n    a();\n} catch (b) {\n    c();\n} finally {\n    d();\n}" },
            { input: '(xx)()', comment: ' magic function call'},
            { input: 'a[1]()', comment: 'another magic function call'},
            { input: 'if(a){b();}else if(c) foo();', output: "if (a) {\n    b();\n} else if (c) foo();" },
            { input: 'switch(x) {case 0: case 1: a(); break; default: break}', output: "switch (x) {\n    case 0:\n    case 1:\n        a();\n        break;\n    default:\n        break\n}" },
            { input: 'switch(x){case -1:break;case !y:break;}', output: 'switch (x) {\n    case -1:\n        break;\n    case !y:\n        break;\n}' },
            { input: 'a !== b' },
            { input: 'if (a) b(); else c();', output: "if (a) b();\nelse c();" },
            { input: "// comment\n(function something() {})", comment: 'typical greasemonkey start' },
            { input: "{\n\n    x();\n\n}", comment: 'duplicating newlines' },
            { input: 'if (a in b) foo();' },
            { input: 'if(X)if(Y)a();else b();else c();',
                output: "if (X)\n    if (Y) a();\n    else b();\nelse c();" },
            { input: 'if (foo) bar();\nelse break' },
            { unchanged: 'var a, b;' },
            { unchanged: 'var a = new function();' },
            { fragment: true, unchanged: 'new function' },
            { unchanged: 'var a, b' },
            { input: '{a:1, b:2}', output: "{\n    a: 1,\n    b: 2\n}" },
            { input: 'a={1:[-1],2:[+1]}', output: 'a = {\n    1: [-1],\n    2: [+1]\n}' },
            { input: "var l = {\\'a\\':\\'1\\', \\'b\\':\\'2\\'}", output: "var l = {\n    \\'a\\': \\'1\\',\n    \\'b\\': \\'2\\'\n}" },
            { input: 'if (template.user[n] in bk) foo();' },
            { unchanged: 'return 45' },
            { unchanged: 'return this.prevObject ||\n\n    this.constructor(null);' },
            { unchanged: 'If[1]' },
            { unchanged: 'Then[1]' },
            { unchanged: 'a = 1e10' },
            { unchanged: 'a = 1.3e10' },
            { unchanged: 'a = 1.3e-10' },
            { unchanged: 'a = -1.3e-10' },
            { unchanged: 'a = 1e-10' },
            { unchanged: 'a = e - 10' },
            { input: 'a = 11-10', output: "a = 11 - 10" },
            { input: "a = 1;// comment", output: "a = 1; // comment" },
            { unchanged: "a = 1; // comment" },
            { input: "a = 1;\n // comment", output: "a = 1;\n// comment" },
            { unchanged: 'a = [-1, -1, -1]' },


            { comment: 'The exact formatting these should have is open for discussion, but they are at least reasonable',
                unchanged: 'a = [ // comment\n    -1, -1, -1\n]' },
            { unchanged: 'var a = [ // comment\n    -1, -1, -1\n]' },
            { unchanged: 'a = [ // comment\n    -1, // comment\n    -1, -1\n]' },
            { unchanged: 'var a = [ // comment\n    -1, // comment\n    -1, -1\n]' },

            { input: 'o = [{a:b},{c:d}]', output: 'o = [{\n    a: b\n}, {\n    c: d\n}]' },

            { comment: 'was: extra space appended',
                input: "if (a) {\n    do();\n}" },

            { comment: 'if/else statement with empty body',
                input: "if (a) {\n// comment\n}else{\n// comment\n}", output: "if (a) {\n    // comment\n} else {\n    // comment\n}" },
            { comment: 'multiple comments indentation', input: "if (a) {\n// comment\n// comment\n}", output: "if (a) {\n    // comment\n    // comment\n}" },
            { input: "if (a) b() else c();", output: "if (a) b()\nelse c();" },
            { input: "if (a) b() else if c() d();", output: "if (a) b()\nelse if c() d();" },

            { unchanged: "{}" },
            { unchanged: "{\n\n}" },
            { input: "do { a(); } while ( 1 );", output: "do {\n    a();\n} while (1);" },
            { unchanged: "do {} while (1);" },
            { input: "do {\n} while (1);", output: "do {} while (1);" },
            { unchanged: "do {\n\n} while (1);" },
            { unchanged: "var a = x(a, b, c)" },
            { input: "delete x if (a) b();", output: "delete x\nif (a) b();" },
            { input: "delete x[x] if (a) b();", output: "delete x[x]\nif (a) b();" },
            { input: "for(var a=1,b=2)d", output: "for (var a = 1, b = 2) d" },
            { input: "for(var a=1,b=2,c=3) d", output: "for (var a = 1, b = 2, c = 3) d" },
            { input: "for(var a=1,b=2,c=3;d<3;d++)\ne", output: "for (var a = 1, b = 2, c = 3; d < 3; d++)\n    e" },
            { input: "function x(){(a||b).c()}", output: "function x() {\n    (a || b).c()\n}" },
            { input: "function x(){return - 1}", output: "function x() {\n    return -1\n}" },
            { input: "function x(){return ! a}", output: "function x() {\n    return !a\n}" },
            { unchanged: "x => x" },
            { unchanged: "(x) => x" },
            { input: "x => { x }", output: "x => {\n    x\n}" },
            { input: "(x) => { x }", output: "(x) => {\n    x\n}" },

            { comment: 'a common snippet in jQuery plugins',
                input_: "settings = $.extend({},defaults,settings);",
                output: "settings = $.extend({}, defaults, settings);" },

            // reserved words used as property names
            { unchanged: "$http().then().finally().default()" },
            { input: "$http()\n.then()\n.finally()\n.default()", output: "$http()\n    .then()\n    .finally()\n    .default()" },
            { unchanged: "$http().when.in.new.catch().throw()" },
            { input: "$http()\n.when\n.in\n.new\n.catch()\n.throw()", output: "$http()\n    .when\n    .in\n    .new\n    .catch()\n    .throw()" },

            { input: '{xxx;}()', output: '{\n    xxx;\n}()' },

            { unchanged: "a = \\'a\\'\nb = \\'b\\'" },
            { unchanged: "a = /reg/exp" },
            { unchanged: "a = /reg/" },
            { unchanged: '/abc/.test()' },
            { unchanged: '/abc/i.test()' },
            { input: "{/abc/i.test()}", output: "{\n    /abc/i.test()\n}" },
            { input: 'var x=(a)/a;', output: 'var x = (a) / a;' },

            { unchanged: 'x != -1' },

            { input: 'for (; s-->0;)t', output: 'for (; s-- > 0;) t' },
            { input: 'for (; s++>0;)u', output: 'for (; s++ > 0;) u' },
            { input: 'a = s++>s--;', output: 'a = s++ > s--;' },
            { input: 'a = s++>--s;', output: 'a = s++ > --s;' },

            { input: '{x=#1=[]}', output: '{\n    x = #1=[]\n}' },
            { input: '{a:#1={}}', output: '{\n    a: #1={}\n}' },
            { input: '{a:#1#}', output: '{\n    a: #1#\n}' },

            { fragment: true, unchanged: '"incomplete-string' },
            { fragment: true, unchanged: "\\'incomplete-string" },
            { fragment: true, unchanged: '/incomplete-regex' },
            { fragment: true, unchanged: '`incomplete-template-string' },

            { fragment: true, input: '{a:1},{a:2}', output: '{\n    a: 1\n}, {\n    a: 2\n}' },
            { fragment: true, input: 'var ary=[{a:1}, {a:2}];', output: 'var ary = [{\n    a: 1\n}, {\n    a: 2\n}];' },

            { comment: 'incomplete', fragment: true, input: '{a:#1', output: '{\n    a: #1' },
            { comment: 'incomplete', fragment: true, input: '{a:#', output: '{\n    a: #' },

            { comment: 'incomplete', fragment: true, input: '}}}', output: '}\n}\n}' },

            { fragment: true, unchanged: '<!--\nvoid();\n// -->' },

            { comment: 'incomplete regexp', fragment: true, input: 'a=/regexp', output: 'a = /regexp' },

            { input: '{a:#1=[],b:#1#,c:#999999#}', output: '{\n    a: #1=[],\n    b: #1#,\n    c: #999999#\n}' },

            { unchanged: "a = 1e+2" },
            { unchanged: "a = 1e-2" },
            { input: "do{x()}while(a>1)", output: "do {\n    x()\n} while (a > 1)" },

            { input: "x(); /reg/exp.match(something)", output: "x();\n/reg/exp.match(something)" },

            { fragment: true, input: "something();(", output: "something();\n(" },
            { fragment: true, input: "#!she/bangs, she bangs\nf=1", output: "#!she/bangs, she bangs\n\nf = 1" },
            { fragment: true, input: "#!she/bangs, she bangs\n\nf=1", output: "#!she/bangs, she bangs\n\nf = 1" },
            { fragment: true, unchanged: "#!she/bangs, she bangs\n\n/* comment */" },
            { fragment: true, unchanged: "#!she/bangs, she bangs\n\n\n/* comment */" },
            { fragment: true, unchanged: "#" },
            { fragment: true, unchanged: "#!" },

            { input: "function namespace::something()" },

            { fragment: true, unchanged: "<!--\nsomething();\n-->" },
            { fragment: true, input: "<!--\nif(i<0){bla();}\n-->", output: "<!--\nif (i < 0) {\n    bla();\n}\n-->" },

            { input: '{foo();--bar;}', output: '{\n    foo();\n    --bar;\n}' },
            { input: '{foo();++bar;}', output: '{\n    foo();\n    ++bar;\n}' },
            { input: '{--bar;}', output: '{\n    --bar;\n}' },
            { input: '{++bar;}', output: '{\n    ++bar;\n}' },
            { input: 'if(true)++a;', output: 'if (true) ++a;' },
            { input: 'if(true)\n++a;', output: 'if (true)\n    ++a;' },
            { input: 'if(true)--a;', output: 'if (true) --a;' },
            { input: 'if(true)\n--a;', output: 'if (true)\n    --a;' },
            { unchanged: 'elem[array]++;' },
            { unchanged: 'elem++ * elem[array]++;' },
            { unchanged: 'elem-- * -elem[array]++;' },
            { unchanged: 'elem-- + elem[array]++;' },
            { unchanged: 'elem-- - elem[array]++;' },
            { unchanged: 'elem-- - -elem[array]++;' },
            { unchanged: 'elem-- - +elem[array]++;' },


            { comment: 'Handling of newlines around unary ++ and -- operators',
                input: '{foo\n++bar;}', output: '{\n    foo\n    ++bar;\n}' },
            { input: '{foo++\nbar;}', output: '{\n    foo++\n    bar;\n}' },

            { comment: 'This is invalid, but harder to guard against. Issue #203.',
                input: '{foo\n++\nbar;}', output: '{\n    foo\n    ++\n    bar;\n}' },

            { comment: 'regexps',
                input: 'a(/abc\\\\/\\\\/def/);b()', output: "a(/abc\\\\/\\\\/def/);\nb()" },
            { input: 'a(/a[b\\\\[\\\\]c]d/);b()', output: "a(/a[b\\\\[\\\\]c]d/);\nb()" },
            { comment: 'incomplete char class', fragment: true, unchanged: 'a(/a[b\\\\[' },

            { comment: 'allow unescaped / in char classes',
                input: 'a(/[a/b]/);b()', output: "a(/[a/b]/);\nb()" },
            { unchanged: 'typeof /foo\\\\//;' },
            { unchanged: 'yield /foo\\\\//;' },
            { unchanged: 'throw /foo\\\\//;' },
            { unchanged: 'do /foo\\\\//;' },
            { unchanged: 'return /foo\\\\//;' },
            { unchanged: 'switch (a) {\n    case /foo\\\\//:\n        b\n}' },
            { unchanged: 'if (a) /foo\\\\//\nelse /foo\\\\//;' },

            { unchanged: 'if (foo) /regex/.test();' },
            { unchanged: "for (index in [1, 2, 3]) /^test$/i.test(s)"},
            { unchanged: 'result = yield pgClient.query_(queryString);' },

            { unchanged: 'function foo() {\n    return [\n        "one",\n        "two"\n    ];\n}' },
            { input: 'a=[[1,2],[4,5],[7,8]]', output: "a = [\n    [1, 2],\n    [4, 5],\n    [7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],function(){},[7,8]]',
                output: "a = [\n    [1, 2],\n    [4, 5],\n    function() {},\n    [7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],function(){},function(){},[7,8]]',
                output: "a = [\n    [1, 2],\n    [4, 5],\n    function() {},\n    function() {},\n    [7, 8]\n]" },
            { input: 'a=[[1,2],[4,5],function(){},[7,8]]',
                output: "a = [\n    [1, 2],\n    [4, 5],\n    function() {},\n    [7, 8]\n]" },
            { input: 'a=[b,c,function(){},function(){},d]',
                output: "a = [b, c, function() {}, function() {}, d]" },
            { input: 'a=[b,c,\nfunction(){},function(){},d]',
                output: "a = [b, c,\n    function() {},\n    function() {},\n    d\n]" },
            { input: 'a=[a[1],b[4],c[d[7]]]', output: "a = [a[1], b[4], c[d[7]]]" },
            { input: '[1,2,[3,4,[5,6],7],8]', output: "[1, 2, [3, 4, [5, 6], 7], 8]" },

            { input: '[[["1","2"],["3","4"]],[["5","6","7"],["8","9","0"]],[["1","2","3"],["4","5","6","7"],["8","9","0"]]]',
              output: '[\n    [\n        ["1", "2"],\n        ["3", "4"]\n    ],\n    [\n        ["5", "6", "7"],\n        ["8", "9", "0"]\n    ],\n    [\n        ["1", "2", "3"],\n        ["4", "5", "6", "7"],\n        ["8", "9", "0"]\n    ]\n]' },

            { input: '{[x()[0]];indent;}', output: '{\n    [x()[0]];\n    indent;\n}' },
            { unchanged: '/*\n foo trailing space    \n * bar trailing space   \n**/'},
            { unchanged: '{\n    /*\n    foo    \n    * bar    \n    */\n}'},

            { unchanged: 'return ++i' },
            { unchanged: 'return !!x' },
            { unchanged: 'return !x' },
            { input: 'return [1,2]', output: 'return [1, 2]' },
            { unchanged: 'return;' },
            { unchanged: 'return\nfunc' },
            { input: 'catch(e)', output: 'catch (e)' },
            { unchanged: 'yield [1, 2]' },

            { input: 'var a=1,b={foo:2,bar:3},{baz:4,wham:5},c=4;',
                output: 'var a = 1,\n    b = {\n        foo: 2,\n        bar: 3\n    },\n    {\n        baz: 4,\n        wham: 5\n    }, c = 4;' },
            { input: 'var a=1,b={foo:2,bar:3},{baz:4,wham:5},\nc=4;',
                output: 'var a = 1,\n    b = {\n        foo: 2,\n        bar: 3\n    },\n    {\n        baz: 4,\n        wham: 5\n    },\n    c = 4;' },

            {
                comment: 'inline comment',
                input_: 'function x(/*int*/ start, /*string*/ foo)',
                output: 'function x( /*int*/ start, /*string*/ foo)'
            },

            { comment: 'javadoc comment',
                input: '/**\n* foo\n*/', output: '/**\n * foo\n */' },
            { input: '{\n/**\n* foo\n*/\n}', output: '{\n    /**\n     * foo\n     */\n}' },

            { comment: 'starless block comment',
                unchanged: '/**\nfoo\n*/' },
            { unchanged: '/**\nfoo\n**/' },
            { unchanged: '/**\nfoo\nbar\n**/' },
            { unchanged: '/**\nfoo\n\nbar\n**/' },
            { unchanged: '/**\nfoo\n    bar\n**/' },
            { input: '{\n/**\nfoo\n*/\n}', output: '{\n    /**\n    foo\n    */\n}' },
            { input: '{\n/**\nfoo\n**/\n}', output: '{\n    /**\n    foo\n    **/\n}' },
            { input: '{\n/**\nfoo\nbar\n**/\n}', output: '{\n    /**\n    foo\n    bar\n    **/\n}' },
            { input: '{\n/**\nfoo\n\nbar\n**/\n}', output: '{\n    /**\n    foo\n\n    bar\n    **/\n}' },
            { input: '{\n/**\nfoo\n    bar\n**/\n}', output: '{\n    /**\n    foo\n        bar\n    **/\n}' },
            { unchanged: '{\n    /**\n    foo\nbar\n    **/\n}' },

            { input: 'var a,b,c=1,d,e,f=2;', output: 'var a, b, c = 1,\n    d, e, f = 2;' },
            { input: 'var a,b,c=[],d,e,f=2;', output: 'var a, b, c = [],\n    d, e, f = 2;' },
            { unchanged: 'function() {\n    var a, b, c, d, e = [],\n        f;\n}' },

            { input: 'do/regexp/;\nwhile(1);', output: 'do /regexp/;\nwhile (1);' },
            { input: 'var a = a,\na;\nb = {\nb\n}', output: 'var a = a,\n    a;\nb = {\n    b\n}' },

            { unchanged: 'var a = a,\n    /* c */\n    b;' },
            { unchanged: 'var a = a,\n    // c\n    b;' },

            { comment: 'weird element referencing',
                unchanged: 'foo.("bar");' },


            { unchanged: 'if (a) a()\nelse b()\nnewline()' },
            { unchanged: 'if (a) a()\nnewline()' },
            { input: 'a=typeof(x)', output: 'a = typeof(x)' },

            { unchanged: 'var a = function() {\n        return null;\n    },\n    b = false;' },

            { unchanged: 'var a = function() {\n    func1()\n}' },
            { unchanged: 'var a = function() {\n    func1()\n}\nvar b = function() {\n    func2()\n}' },

            { comment: 'code with and without semicolons',
                input_: 'var whatever = require("whatever");\nfunction() {\n    a = 6;\n}',
                output: 'var whatever = require("whatever");\n\nfunction() {\n    a = 6;\n}' },
            { input: 'var whatever = require("whatever")\nfunction() {\n    a = 6\n}',
                 output: 'var whatever = require("whatever")\n\nfunction() {\n    a = 6\n}' },

            { input: '{"x":[{"a":1,"b":3},\n7,8,8,8,8,{"b":99},{"a":11}]}', output: '{\n    "x": [{\n            "a": 1,\n            "b": 3\n        },\n        7, 8, 8, 8, 8, {\n            "b": 99\n        }, {\n            "a": 11\n        }\n    ]\n}' },
            { input: '{"x":[{"a":1,"b":3},7,8,8,8,8,{"b":99},{"a":11}]}', output: '{\n    "x": [{\n        "a": 1,\n        "b": 3\n    }, 7, 8, 8, 8, 8, {\n        "b": 99\n    }, {\n        "a": 11\n    }]\n}' },

            { input: '{"1":{"1a":"1b"},"2"}', output: '{\n    "1": {\n        "1a": "1b"\n    },\n    "2"\n}' },
            { input: '{a:{a:b},c}', output: '{\n    a: {\n        a: b\n    },\n    c\n}' },

            { input: '{[y[a]];keep_indent;}', output: '{\n    [y[a]];\n    keep_indent;\n}' },

            { input: 'if (x) {y} else { if (x) {y}}', output: 'if (x) {\n    y\n} else {\n    if (x) {\n        y\n    }\n}' },

            { unchanged: 'if (foo) one()\ntwo()\nthree()' },
            { unchanged: 'if (1 + foo() && bar(baz()) / 2) one()\ntwo()\nthree()' },
            { unchanged: 'if (1 + foo() && bar(baz()) / 2) one();\ntwo();\nthree();' },

            { input: 'var a=1,b={bang:2},c=3;', output: 'var a = 1,\n    b = {\n        bang: 2\n    },\n    c = 3;' },
            { input: 'var a={bing:1},b=2,c=3;', output: 'var a = {\n        bing: 1\n    },\n    b = 2,\n    c = 3;' },

        ],
    }],
    // Example
    examples: [{
        group_name: "one",
        description: "",
        options: [],
        values: [
            {
                source: "", //string or array of lines
                output: ""  //string or array of lines
            }
        ]
    }]
}
