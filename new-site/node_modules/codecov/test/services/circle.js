var circle = require("../../lib/services/circle");

describe("Circle CI Provider", function(){

  it ("can detect circle", function(){
    process.env.CIRCLECI = "true";
    expect(circle.detect()).to.be(true);
  });

  it ("can get circle env info get_commit_status", function(){
    process.env.CIRCLECI = 'true';
    process.env.CIRCLE_BUILD_NUM = '1234';
    process.env.CIRCLE_SHA1 = '5678';
    process.env.CIRCLE_BRANCH = 'master';
    process.env.CIRCLE_NODE_INDEX = '1';
    process.env.CIRCLE_PR_NUMBER = 'blah';
    process.env.CIRCLE_PROJECT_USERNAME = 'owner';
    process.env.CIRCLE_PROJECT_REPONAME = 'repo';
    expect(circle.configuration()).to.eql({
      service : 'circleci',
      commit : '5678',
      build : '1234.1',
      job : '1234.1',
      branch : 'master',
      pr : 'blah',
      slug : 'owner/repo'
    });
  });

});
