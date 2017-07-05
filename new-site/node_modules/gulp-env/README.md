gulp-env
========

Add env vars to your process.env


Install
========

```
npm i --save-dev gulp-env
```

Usage
========

##Quick Example

```
//gulpfile.js
gulp = require('gulp'),
  nodemon = require('nodemon'),
  env = require('gulp-env');

gulp.task('nodemon', function() {
	//nodemon server ...
});

gulp.task('set-env', function () {
	env({
		file: ".env.json",
		vars: {
			//any vars you want to overwrite
		}
	});
});

gulp.task('default', ['set-env', 'nodemon'])
```

##The Details

gulp-env handles two options: `file` and `vars`.

###options.file

The `file` option uses `require()` under the hood to pull in data and assign it to
the `process.env`. `gulp-env` has test coverage for two options: JSON, and a JS
object exported as a module.

```
//.env.json
{
	MONGO_URI: "mongodb://localhost:27017/testdb
}

//.env
module.exports = {
	MONGO_URI: "mongodb://localhost:27017/testdb
}

//gulpfile.js
var env = require('gulp-env')
env({
	file: ".env"
	//OR
	file: ".env.json"
})
```

###options.vars

Properties passed to the vars option will be set on process.env as well.
These properties will overwrite the external file's properties.

```
//gulpfile.js
var env = require('gulp-env')
env({
	vars: {
		MONGO_URI: "mongodb://localhost:27017/testdb-for-british-eyes-only",
		PORT: 9001
	}
})
```

TODO
========

- handle ini files
