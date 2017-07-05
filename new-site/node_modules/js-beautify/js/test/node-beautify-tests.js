/*global js_beautify: true */
/*jshint node:true */

var SanityTest = require('./sanitytest'),
    Urlencoded = require('../lib/unpackers/urlencode_unpacker'),
    run_javascript_tests = require('./beautify-javascript-tests').run_javascript_tests,
    run_css_tests = require('./beautify-css-tests').run_css_tests,
    run_html_tests = require('./beautify-html-tests').run_html_tests;

function node_beautifier_legacy_tests(name, test_runner) {
    console.log('Testing ' + name + ' with node.js CommonJS (legacy names)...');
    var results = new SanityTest();
    var beautify = require('../index');

    test_runner(
            results,
            Urlencoded,
            beautify.js_beautify,
            beautify.html_beautify,
            beautify.css_beautify);

    console.log(results.results_raw());
    return results;
}

function node_beautifier_tests(name, test_runner) {
    console.log('Testing ' + name + ' with node.js CommonJS (new names)...');
    var results = new SanityTest();
    var beautify = require('../index');


    test_runner(
            results,
            Urlencoded,
            beautify.js,
            beautify.html,
            beautify.css);

    console.log(results.results_raw());
    return results;
}


if (require.main === module) {
    process.exit(
            node_beautifier_tests('js-beautifier', run_javascript_tests).get_exitcode() +
            node_beautifier_legacy_tests('js-beautifier', run_javascript_tests).get_exitcode() +
            node_beautifier_tests('cs-beautifier', run_css_tests).get_exitcode() +
            node_beautifier_legacy_tests('css-beautifier', run_css_tests).get_exitcode() +
            node_beautifier_tests('html-beautifier', run_html_tests).get_exitcode() +
            node_beautifier_legacy_tests('html-beautifier', run_html_tests).get_exitcode()
        );
}
