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
  beforeEach(function(){
    try {
      fs.unlinkSync('.bowerrc');
    } catch (e) {}
  });

  after(function(){
    try {
      fs.unlinkSync('.bowerrc');
    } catch (e) {}
  });

  it("can get a token passed via env variable", function(){
    process.env.codecov_token = 'abc123';
    expect(codecov.upload({options: {dump: true}}).query.token).to.eql('abc123');
    delete process.env.codecov_token;
    process.env.CODECOV_TOKEN = 'ABC123';
    expect(codecov.upload({options: {dump: true}}).query.token).to.eql('ABC123');
  });

  it("can get a token passed in cli", function(){
    expect(codecov.upload({options: {dump: true, token: 'qwerty'}}).query.token).to.eql('qwerty');
  });

  it("can auto detect reports", function(){
    var res = codecov.upload({options: {dump: true}});
    expect(res.files[0].split('/').pop()).to.eql('example.coverage.txt');
    expect(res.body).to.contain('this file is intentionally left blank');
  });

  it("can specify report in cli", function(){
    var res = codecov.upload({options: {dump: true, file: 'test/example.coverage.txt'}});
    expect(res.files[0].split('/').pop()).to.eql('example.coverage.txt');
    expect(res.body).to.contain('this file is intentionally left blank');
  });

  it("can specify report in cli fail", function(){
    var res = codecov.upload({options: {dump: true, file: 'notreal.txt'}});
    expect(res.debug).to.contain('failed: notreal.txt');
  });

  it("can detect .bowerrc with directory", function(){
    fs.writeFileSync('.bowerrc', '{"directory": "test"}');
    var res = codecov.upload({options: {dump: true}});
    expect(res.files).to.eql([]);
  });

  it("can detect .bowerrc without directory", function(){
    fs.writeFileSync('.bowerrc', '{"key": "value"}');
    var res = codecov.upload({options: {dump: true}});
    expect(res.files[0].split('/').pop()).to.eql('example.coverage.txt');
    expect(res.body).to.contain('this file is intentionally left blank');
  });

  it("can disable search", function(){
    var res = codecov.upload({options: {dump: true, disable: 'search'}});
    expect(res.debug).to.contain('disabled search');
    expect(res.files).to.eql([]);
  });

  it("can disable gcov", function(){
    var res = codecov.upload({options: {dump: true, disable: 'gcov'}});
    console.log(res.debug);
    expect(res.debug).to.contain('disabled gcov');
  });

  it("can disable detection", function(){
    var res = codecov.upload({options: {dump: true, disable: 'detect'}});
    expect(res.debug).to.contain('disabled detect');
  });

  it("can get build from cli args", function(){
    var res = codecov.upload({options: {dump: true, build: 'value'}});
    expect(res.query.build).to.eql('value');
  });

  it("can get commit from cli args", function(){
    var res = codecov.upload({options: {dump: true, commit: 'value'}});
    expect(res.query.commit).to.eql('value');
  });

  it("can get branch from cli args", function(){
    var res = codecov.upload({options: {dump: true, branch: 'value'}});
    expect(res.query.branch).to.eql('value');
  });

  it("can get slug from cli args", function(){
    var res = codecov.upload({options: {dump: true, slug: 'value'}});
    expect(res.query.slug).to.eql('value');
  });

  it("can include env in cli", function(){
    process.env.HELLO = 'world';
    var res = codecov.upload({options: {dump: true, env: 'HELLO,VAR1'}});
    expect(res.body).to.contain('HELLO=world\n');
    expect(res.body).to.contain('VAR1=\n');
  });

  it("can include env in env", function(){
    process.env.HELLO = 'world';
    process.env.CODECOV_ENV = 'HELLO,VAR1';
    var res = codecov.upload({options: {dump: true, env: 'VAR2'}});
    expect(res.body).to.contain('HELLO=world\n');
    expect(res.body).to.contain('VAR1=\n');
    expect(res.body).to.contain('VAR2=\n');
  });

  it("can have custom args for gcov", function(){
    var res = codecov.upload({options: {dump: true,
                                        'gcov-root': 'folder/path',
                                        'gcov-glob': 'ignore/this/folder',
                                        'gcov-exec': 'llvm-gcov',
                                        'gcov-args': '-o'}});
    expect(res.debug).to.contain('find folder/path -type f -name \'*.gcno\' -not -path \'ignore/this/folder\' -exec llvm-gcov -o {} +');
  });


});
