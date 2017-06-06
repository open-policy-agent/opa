'use strict';

var path = require('path'),
	gutil = require('gulp-util'),
	consolidate = require('consolidate'),
	_ = require('lodash'),
	Stream = require('stream');

var PLUGIN_NAME  = 'gulp-iconfont-css';

function iconfontCSS(config) {
	var glyphMap = [],
		currentGlyph,
		currentCodePoint,
		inputFilePrefix,
		stream,
		outputFile,
		engine;

	// Set default values
	config = _.merge({
		path: 'css',
		targetPath: '_icons.css',
		fontPath: './',
		engine: 'lodash',
		firstGlyph: 0xE001
	}, config);

	// Enable default stylesheet generators
	if(!config.path) {
		config.path = 'scss';
	}
	if(/^(scss|less|css)$/i.test(config.path)) {
		config.path = __dirname + '/templates/_icons.' + config.path;
	}

	// Validate config
	if (!config.fontName) {
		throw new gutil.PluginError(PLUGIN_NAME, 'Missing option "fontName"');
	}
	if (!consolidate[config.engine]) {
		throw new gutil.PluginError(PLUGIN_NAME, 'Consolidate missing template engine "' + config.engine + '"');
	}
	try {
		engine = require(config.engine);
	} catch(e) {
		throw new gutil.PluginError(PLUGIN_NAME, 'Template engine "' + config.engine + '" not present');
	}

	// Define starting point
	currentGlyph = config.firstGlyph;

	// Happy streaming
	stream = Stream.PassThrough({
		objectMode: true
	});

	stream._transform = function(file, unused, cb) {
		if (file.isNull()) {
			this.push(file);
			return cb();
		}

		// Create output file
		if (!outputFile) {
			outputFile = new gutil.File({
				base: file.base,
				cwd: file.cwd,
				path: path.join(file.base, config.targetPath),
				contents: file.isBuffer() ? new Buffer(0) : new Stream.PassThrough()
			});
		}

		currentCodePoint = currentGlyph.toString(16).toUpperCase();

		// Add glyph
		glyphMap.push({
			fileName: path.basename(file.path, '.svg'),
			codePoint: currentCodePoint
		});

		// Prepend codePoint to input file path for gulp-iconfont
		inputFilePrefix = 'u' + currentCodePoint + '-';

		file.path = path.dirname(file.path) + '/' + inputFilePrefix + path.basename(file.path);

		// Increase counter
		currentGlyph++;

		this.push(file);
		cb();
	};

	stream._flush = function(cb) {
		var content;

		if (glyphMap.length) {
			consolidate[config.engine](config.path, {
					glyphs: glyphMap,
					fontName: config.fontName,
					fontPath: config.fontPath
				}, function(error, html) {
					if (error) {
						throw error;
					}

					content = Buffer(html);

					if (outputFile.isBuffer()) {
						outputFile.contents = content;
					} else {
						outputFile.contents.write(content);
						outputFile.contents.end();
					}

					stream.push(outputFile);

					cb();
			});
		} else {
			cb();
		}
	};

	return stream;
};

module.exports = iconfontCSS;
