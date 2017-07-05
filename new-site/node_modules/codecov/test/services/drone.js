var drone = require("../../lib/services/drone");
var execSync = require('child_process').execSync;
if (!execSync) {
  var exec = require('execSync').exec;
  var execSync = function(cmd){
    return exec(cmd).stdout;
  };
}

describe("Drone.io CI Provider", function(){

  it ("can detect drone", function(){
    process.env.DRONE = "true";
    expect(drone.detect()).to.be(true);
  });

  it ("can get drone env info", function(){
    process.env.DRONE_BUILD_NUMBER = "1234";
    process.env.DRONE_BRANCH = "master";
    process.env.DRONE_BUILD_URL = 'https://...';
    process.env.DRONE_BUILD_DIR = '/';
    expect(drone.configuration()).to.eql({
      service : 'drone.io',
      commit : execSync("git rev-parse HEAD || hg id -i --debug | tr -d '+'").toString().trim(),
      build : '1234',
      root : '/',
      branch : 'master',
      build_url : 'https://...'
    });
  });

});
