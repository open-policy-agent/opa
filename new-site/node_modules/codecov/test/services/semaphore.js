var semaphore = require("../../lib/services/semaphore");

describe("Semaphore CI Provider", function(){

  it ("can detect semaphore", function(){
    process.env.SEMAPHORE = "true";
    expect(semaphore.detect()).to.be(true);
  });

  it ("can get semaphore env info", function(){
    process.env.SEMAPHORE_BUILD_NUMBER = "1234";
    process.env.REVISION = "5678";
    process.env.SEMAPHORE_CURRENT_THREAD = "1";
    process.env.BRANCH_NAME = "master";
    process.env.SEMAPHORE_REPO_SLUG = "owner/repo";
    expect(semaphore.configuration()).to.eql({
      service : 'semaphore',
      commit : '5678',
      build : '1234.1',
      branch : 'master',
      slug : 'owner/repo'
    });
  });

});
