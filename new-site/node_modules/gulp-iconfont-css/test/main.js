var gulp = require('gulp'),
	fs = require('fs'),
	es = require('event-stream'),
	assert = require('assert'),
	iconfontCss = require('../');

describe('gulp-iconfont-css', function() {
	function testType(type, name) {
		var resultsDir = __dirname + '/results_'+type;
		it('should generate '+name+' file', function(done) {
			gulp.src(__dirname + '/fixtures/icons/*.svg')
				.pipe(iconfontCss({
					fontName: 'Icons',
					path: type,
					targetPath: '../_icons.'+type
				}))
				.pipe(gulp.dest(resultsDir + '/icons/'))
				.pipe(es.wait(function() {
					assert.equal(
						fs.readFileSync(resultsDir + '/_icons.'+type, 'utf8'),
						fs.readFileSync(__dirname + '/expected/_icons.'+type, 'utf8')
					);

					fs.unlinkSync(resultsDir + '/_icons.'+type);
					fs.unlinkSync(resultsDir + '/icons/uE001-github.svg');
					fs.unlinkSync(resultsDir + '/icons/uE002-twitter.svg');
					fs.rmdirSync(resultsDir + '/icons/');
					fs.rmdirSync(resultsDir);

					done();
				}));
		});
	}

	testType('scss', 'SCSS');
	testType('less', 'Less');
	testType('css', 'CSS');

});