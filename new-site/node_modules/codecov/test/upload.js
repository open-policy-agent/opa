var fs = require('fs');
var codecov = require("../lib/codecov");
var execSync = require('child_process').execSync;
if (!execSync) {
  var exec = require('execSync').exec;
  var execSync = function(cmd){
    return exec(cmd).stdout;
  };
}


describe("Codecov", function(){
  it("can get upload to v2", function(done){
    codecov.sendToCodecovV2('https://codecov.io',
                            {
                              token: '473c8c5b-10ee-4d83-86c6-bfd72a185a27',
                              commit: 'c739768fcac68144a3a6d82305b9c4106934d31a',
                              branch: 'master'
                            },
                            'testing node-'+codecov.version,
                            function(body){
                              expect(body).to.contain('http://codecov.io/github/codecov/ci-repo?ref=c739768fcac68144a3a6d82305b9c4106934d31a');
                              done();
                            },
                            function(err){
                              throw err;
                            });
  });

  it("can get upload to v3", function(done){
    codecov.sendToCodecovV3('https://codecov.io',
                            {
                              token: '473c8c5b-10ee-4d83-86c6-bfd72a185a27',
                              commit: 'c739768fcac68144a3a6d82305b9c4106934d31a',
                              branch: 'master'
                            },
                            'testing node-'+codecov.version,
                            function(body){
                              expect(body).to.contain('http://codecov.io/github/codecov/ci-repo?ref=c739768fcac68144a3a6d82305b9c4106934d31a');
                              done();
                            },
                            function(err){
                              throw err;
                            });
  });

});
