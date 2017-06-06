#!/usr/bin/env node
var fs = require('fs');
var mustache = require('mustache');

function generate_tests() {
    var test_data, template;

    // javascript
    test_data = require(__dirname + '/data/javascript.js').test_data;
    set_formatters(test_data, 'bt', '// ')
    template = fs.readFileSync(__dirname + '/template/node-javascript.mustache', {encoding: 'utf-8'});
    fs.writeFileSync(__dirname + '/../js/test/beautify-javascript-tests.js', mustache.render(template, test_data), {encoding: 'utf-8'});

    set_formatters(test_data, 'bt', '# ')
    template = fs.readFileSync(__dirname + '/template/python-javascript.mustache', {encoding: 'utf-8'});
    fs.writeFileSync(__dirname + '/../python/jsbeautifier/tests/testjsbeautifier.py', mustache.render(template, test_data), {encoding: 'utf-8'});

    // css
    test_data = require(__dirname + '/data/css.js').test_data;
    set_formatters(test_data, 't', '// ')
    template = fs.readFileSync(__dirname + '/template/node-css.mustache', {encoding: 'utf-8'});
    fs.writeFileSync(__dirname + '/../js/test/beautify-css-tests.js', mustache.render(template, test_data), {encoding: 'utf-8'});

    set_formatters(test_data, 't', '# ')
    template = fs.readFileSync(__dirname + '/template/python-css.mustache', {encoding: 'utf-8'});
    fs.writeFileSync(__dirname + '/../python/cssbeautifier/tests/test.py', mustache.render(template, test_data), {encoding: 'utf-8'});

    // html
    test_data = require(__dirname + '/data/html.js').test_data;
    set_formatters(test_data, 'bth', '// ')
    template = fs.readFileSync(__dirname + '/template/node-html.mustache', {encoding: 'utf-8'});
    fs.writeFileSync(__dirname + '/../js/test/beautify-html-tests.js', mustache.render(template, test_data), {encoding: 'utf-8'});

    // no python html beautifier, so no tests
}

function isStringOrArray(val) {
    return typeof val === 'string' || val instanceof Array;
}

function getTestString(val) {
    if (typeof val === 'string') {
        return "'" + val.replace(/\n/g,'\\n').replace(/\t/g,'\\t') + "'";
    } else if (val instanceof Array) {
        return  "'" + val.join("\\n' +\n            '").replace(/\t/g,'\\t') + "'";
    } else {
        return null;
    }
}

function set_formatters (data, test_method, comment_mark) {

    // utility mustache functions
    data.matrix_context_string = function() {
        var context = this;
        return function(text, render) {
            var outputs = [];
            // text is ignored for this
            for (var name in context) {
                if (name === 'options') {
                    continue;
                }

                if (context.hasOwnProperty(name)) {
                    outputs.push(name + ' = "' + context[name] + '"');
                }
            }
            return render(outputs.join(', '));
        }
    };

    data.test_line = function() {
        return function(text, render) {
            var method_text = test_method;
            if (this.fragment) {
                method_text = 'test_fragment';
            }

            // text is ignored for this.
            var comment = '';
            if (typeof this.comment === 'string') {
                comment = '\n        ' + comment_mark + this.comment + '\n        ';
            } else if (this.comment instanceof Array) {
                comment = '\n        ' + comment_mark + this.comment.join('\n        ' + comment_mark) + '\n        ';
            }

            var input = null;
            var before_input = method_text + '(';
            var before_output = ', ';

            function set_input(field, opt_newline) {
                if (input !== null && isStringOrArray(field)) {
                    throw "Only one test input field allowed (input, input_, or unchanged): " + input;
                }

                if (typeof field === 'string' && !opt_newline) {
                    input = getTestString(field);
                } else if (field instanceof Array || (typeof field === 'string' && opt_newline)) {
                    before_input = method_text + '(\n            ';
                    before_output = ',\n            ';
                    input = getTestString(field);
                }
            }

            set_input(this.input);

            // allow underscore for formatting alignment with "output"
            set_input(this.input_, true);

            // use "unchanged" instead of "input" if there is no output
            set_input(this.unchanged);
            if(isStringOrArray(this.unchanged) && isStringOrArray(this.output)) {
                throw "Cannot specify 'output' with 'unchanged' test input: " + input;
            }

            if (input === null) {
                throw "Missing test input field (input, input_, or unchanged).";
            }

            var output = null;
            if (typeof this.output === 'string') {
                output = getTestString(this.output);
            } else if (this.output instanceof Array) {
                before_input = method_text + '(\n            ';
                before_output = ',\n            ';
                output = getTestString(this.output);
            } else {
                before_output = '';
            }

            if (input === output) {
                throw "Test strings are identical.  Omit 'output' and use 'unchanged': " + input;
            }

            if(output && output.indexOf('<%') !== -1) {
                mustache.tags = ['<%', '%>'];
            }
            input = render(input);
            output = render(output);
            if(output && output.indexOf('<%') !== -1) {
                mustache.tags = ['{{', '}}'];
            }

            if (output === input) {
                before_output = '';
                output = '';
            }
            return  comment  + before_input + input + before_output + output + ')';
        }
    };

    data.set_mustache_tags = function() {
        return function(text, render) {
            if(this.template) {
                mustache.tags = this.template.split(' ');
            }
            return '';
        }
    };

    data.unset_mustache_tags = function() {
        return function(text, render) {
            if(this.template) {
                mustache.tags = ['{{', '}}'];
            }
            return '';
        }
    };
}

generate_tests();
