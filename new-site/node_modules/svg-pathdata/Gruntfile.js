module.exports = function(grunt) {

    require("matchdep").filterDev("grunt-*").forEach(grunt.loadNpmTasks);
    
    grunt.initConfig({
        clean: ['dist'],
        
        browserify: {
            lib: {
                src: 'src/SVGPathData.js',
                dest: 'dist/SVGPathData.js',
                options: {
                    bundleOptions: {
                       standalone: 'SVGPathData'
                    }
                }
            }
        },

        karma: {
            local: {
                configFile: 'karma.conf.js'
            },
            travis: {
                configFile: 'karma.conf.js',
                singleRun: true,
                browsers: ['Firefox']
            }
        },

        watch: {
            code: {
                files: ['src/*.js'],
                tasks: ['dist']
            }
        },

        parallel: {
            testing: {
                options: {
                  stream: true
                },
                tasks: [{
                  grunt: true,
                  args: ['karma:local']
                },{
                  grunt: true,
                  args: ['watch']
                }]
            }
        }
    });

    grunt.registerTask('dist', [
        'clean',
        'browserify'
    ]);

    grunt.registerTask('test', [
        'dist',
        'parallel:testing'
    ]);

    grunt.registerTask('default', [
        'test'
    ]);

    grunt.registerTask('travis', [
        'dist',
        'karma:travis'
    ]);
};
