exports.test_data = {
    default_options: [
        { name: "indent_size", value: "1" },
        { name: "indent_char", value: "'\\t'" },
        { name: "selector_separator_newline", value: "true" },
        { name: "end_with_newline", value: "false" },
        { name: "newline_between_rules", value: "false"},
    ],
    groups: [{
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
            { fragment: true, input: '   .tabs{}', output: '   .tabs {}{{eof}}' },
            { fragment: true, input: '   \n\n.tabs{}\n\n\n\n', output: '   .tabs {}{{eof}}' },
            { fragment: true, input: '\n', output: '{{eof}}' }
        ],
    }, {
        name: "Empty braces",
        description: "",
        tests: [
            { input: '.tabs{}', output: '.tabs {}' },
            { input: '.tabs { }', output: '.tabs {}' },
            { input: '.tabs    {    }', output: '.tabs {}' },
            // When we support preserving newlines this will need to change
            { input: '.tabs    \n{\n    \n  }', output: '.tabs {}' }
        ],
    }, {
        name: "",
        description: "",
        tests: [
            {
                input:  '#cboxOverlay {\n\tbackground: url(images/overlay.png) repeat 0 0;\n\topacity: 0.9;\n\tfilter: alpha(opacity = 90);\n}',
                output: '#cboxOverlay {\n\tbackground: url(images/overlay.png) repeat 0 0;\n\topacity: 0.9;\n\tfilter: alpha(opacity=90);\n}'
            },
        ],
    }, {
        name: 'Selector Separator',
        description: '',
        matrix: [
            {
                options: [
                    { name: 'selector_separator_newline', value: 'false' },
                    { name: 'selector_separator', value: '" "' }
                ],
                separator: ' ',
                separator1: ' '
            }, {
                options: [
                    { name: 'selector_separator_newline', value: 'false' },
                    { name: 'selector_separator', value: '"  "' }
                ],
                // BUG: #713
                separator: ' ',
                separator1: ' '
            }, {
                options: [
                    { name: 'selector_separator_newline', value: 'true' },
                    { name: 'selector_separator', value: '" "' }
                ],
                separator: '\\n',
                separator1: '\\n\\t'
            }, {
                options: [
                    { name: 'selector_separator_newline', value: 'true' },
                    { name: 'selector_separator', value: '"  "' }
                ],
                separator: '\\n',
                separator1: '\\n\\t'
            }
        ],
        tests: [
            { input: '#bla, #foo{color:green}', output: '#bla,{{separator}}#foo {\n\tcolor: green\n}'},
            { input: '@media print {.tab{}}', output: '@media print {\n\t.tab {}\n}'},
            { input: '@media print {.tab,.bat{}}', output: '@media print {\n\t.tab,{{separator1}}.bat {}\n}'},
            { input: '#bla, #foo{color:black}', output: '#bla,{{separator}}#foo {\n\tcolor: black\n}'},
            { input: 'a:first-child,a:first-child{color:red;div:first-child,div:hover{color:black;}}',
              output: 'a:first-child,{{separator}}a:first-child {\n\tcolor: red;\n\tdiv:first-child,{{separator1}}div:hover {\n\t\tcolor: black;\n\t}\n}'}
        ]
    }, {
        name: "Newline Between Rules",
        description: "",
        matrix: [
            {
                options: [
                    { name: "newline_between_rules", value: "true" }
                ],
                separator: '\\n'
            }, {
                options: [
                    { name: "newline_between_rules", value: "false" }
                ],
                separator: ''
            }
        ],
        tests: [
            { input: '.div {}\n.span {}', output: '.div {}\n{{separator}}.span {}'},
            { input: '.div{}\n   \n.span{}', output: '.div {}\n{{separator}}.span {}'},
            { input: '.div {}    \n  \n.span { } \n', output: '.div {}\n{{separator}}.span {}'},
            { input: '.div {\n    \n} \n  .span {\n }  ', output: '.div {}\n{{separator}}.span {}'},
            { input: '.selector1 {\n\tmargin: 0; /* This is a comment including an url http://domain.com/path/to/file.ext */\n}\n.div{height:15px;}', output: '.selector1 {\n\tmargin: 0;\n\t/* This is a comment including an url http://domain.com/path/to/file.ext */\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
            { input: '.tabs{width:10px;//end of line comment\nheight:10px;//another\n}\n.div{height:15px;}', output: '.tabs {\n\twidth: 10px; //end of line comment\n\theight: 10px; //another\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
            { input: '#foo {\n\tbackground-image: url(foo@2x.png);\n\t@font-face {\n\t\tfont-family: "Bitstream Vera Serif Bold";\n\t\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n\t}\n}\n.div{height:15px;}', output: '#foo {\n\tbackground-image: url(foo@2x.png);\n\t@font-face {\n\t\tfont-family: "Bitstream Vera Serif Bold";\n\t\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n\t}\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
            { input: '@media screen {\n\t#foo:hover {\n\t\tbackground-image: url(foo@2x.png);\n\t}\n\t@font-face {\n\t\tfont-family: "Bitstream Vera Serif Bold";\n\t\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n\t}\n}\n.div{height:15px;}', output: '@media screen {\n\t#foo:hover {\n\t\tbackground-image: url(foo@2x.png);\n\t}\n\t@font-face {\n\t\tfont-family: "Bitstream Vera Serif Bold";\n\t\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n\t}\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
            { input: '@font-face {\n\tfont-family: "Bitstream Vera Serif Bold";\n\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n}\n@media screen {\n\t#foo:hover {\n\t\tbackground-image: url(foo.png);\n\t}\n\t@media screen and (min-device-pixel-ratio: 2) {\n\t\t@font-face {\n\t\t\tfont-family: "Helvetica Neue"\n\t\t}\n\t\t#foo:hover {\n\t\t\tbackground-image: url(foo@2x.png);\n\t\t}\n\t}\n}', output: '@font-face {\n\tfont-family: "Bitstream Vera Serif Bold";\n\tsrc: url("http://developer.mozilla.org/@api/deki/files/2934/=VeraSeBd.ttf");\n}\n{{separator}}@media screen {\n\t#foo:hover {\n\t\tbackground-image: url(foo.png);\n\t}\n\t@media screen and (min-device-pixel-ratio: 2) {\n\t\t@font-face {\n\t\t\tfont-family: "Helvetica Neue"\n\t\t}\n\t\t#foo:hover {\n\t\t\tbackground-image: url(foo@2x.png);\n\t\t}\n\t}\n}'},
            { input: 'a:first-child{color:red;div:first-child{color:black;}}\n.div{height:15px;}', output: 'a:first-child {\n\tcolor: red;\n\tdiv:first-child {\n\t\tcolor: black;\n\t}\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
            { input: 'a:first-child{color:red;div:not(.peq){color:black;}}\n.div{height:15px;}', output: 'a:first-child {\n\tcolor: red;\n\tdiv:not(.peq) {\n\t\tcolor: black;\n\t}\n}\n{{separator}}.div {\n\theight: 15px;\n}'},
        ],
    }, {
        name: "Functions braces",
        description: "",
        tests: [
            { input: '.tabs(){}', output: '.tabs() {}' },
            { input: '.tabs (){}', output: '.tabs () {}' },
            { input: '.tabs (pa, pa(1,2)), .cols { }', output: '.tabs (pa, pa(1, 2)),\n.cols {}' },
            { input: '.tabs(pa, pa(1,2)), .cols { }', output: '.tabs(pa, pa(1, 2)),\n.cols {}' },
            { input: '.tabs (   )   {    }', output: '.tabs () {}' },
            { input: '.tabs(   )   {    }', output: '.tabs() {}' },
            { input: '.tabs  (t, t2)  \n{\n  key: val(p1  ,p2);  \n  }', output: '.tabs (t, t2) {\n\tkey: val(p1, p2);\n}' },
            { input: '.box-shadow(@shadow: 0 1px 3px rgba(0, 0, 0, .25)) {\n\t-webkit-box-shadow: @shadow;\n\t-moz-box-shadow: @shadow;\n\tbox-shadow: @shadow;\n}' }
        ],
    }, {
        name: "Comments",
        description: "",
        tests: [
            { unchanged: '/* test */' },
            { input: '.tabs{/* test */}', output: '.tabs {\n\t/* test */\n}' },
            { input: '.tabs{/* test */}', output: '.tabs {\n\t/* test */\n}' },
            { input: '/* header */.tabs {}', output: '/* header */\n\n.tabs {}' },
            { input: '.tabs {\n/* non-header */\nwidth:10px;}', output: '.tabs {\n\t/* non-header */\n\twidth: 10px;\n}' },
            { unchanged: '/* header' },
            { unchanged: '// comment' },
            { input: '.selector1 {\n\tmargin: 0; /* This is a comment including an url http://domain.com/path/to/file.ext */\n}',
                output: '.selector1 {\n\tmargin: 0;\n\t/* This is a comment including an url http://domain.com/path/to/file.ext */\n}'},

            { comment: "single line comment support (less/sass)",
                input:'.tabs{\n// comment\nwidth:10px;\n}', output: '.tabs {\n\t// comment\n\twidth: 10px;\n}' },
            { input: '.tabs{// comment\nwidth:10px;\n}', output: '.tabs {\n\t// comment\n\twidth: 10px;\n}' },
            { input: '//comment\n.tabs{width:10px;}', output: '//comment\n.tabs {\n\twidth: 10px;\n}' },
            { input: '.tabs{//comment\n//2nd single line comment\nwidth:10px;}', output: '.tabs {\n\t//comment\n\t//2nd single line comment\n\twidth: 10px;\n}' },
            { input: '.tabs{width:10px;//end of line comment\n}', output: '.tabs {\n\twidth: 10px; //end of line comment\n}' },
            { input: '.tabs{width:10px;//end of line comment\nheight:10px;}', output: '.tabs {\n\twidth: 10px; //end of line comment\n\theight: 10px;\n}' },
            { input: '.tabs{width:10px;//end of line comment\nheight:10px;//another\n}', output: '.tabs {\n\twidth: 10px; //end of line comment\n\theight: 10px; //another\n}' }
        ],
    }, {
        name: "Psuedo-classes vs Variables",
        description: "",
        tests: [
            { unchanged: '@page :first {}' },
            { comment: "Assume the colon goes with the @name. If we're in LESS, this is required regardless of the at-string.",
              input: '@page:first {}',
              output: '@page: first {}' },
            { unchanged: '@page: first {}' }
        ],
    }, {
        name: "SASS/SCSS",
        description: "",
        tests: [
            { comment: "Basic Interpolation",
              unchanged: 'p {\n\t$font-size: 12px;\n\t$line-height: 30px;\n\tfont: #{$font-size}/#{$line-height};\n}'},
            { unchanged: 'p.#{$name} {}' },
            { unchanged: [
                '@mixin itemPropertiesCoverItem($items, $margin) {',
                '\twidth: calc((100% - ((#{$items} - 1) * #{$margin}rem)) / #{$items});',
                '\tmargin: 1.6rem #{$margin}rem 1.6rem 0;',
                '}'
            ]}
        ],
    }, {

    }]
}
