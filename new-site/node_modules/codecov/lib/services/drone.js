var execSync = require('child_process').execSync;
if (!execSync) {
  var exec = require('execSync').exec;
  var execSync = function(cmd){
    return exec(cmd).stdout;
  };
}

module.exports = {

  detect : function(){
    return !!process.env.DRONE;
  },

  configuration : function(){
    console.log('    Drone.io CI Detected');
    return {
      service : 'drone.io',
      build : process.env.DRONE_BUILD_NUMBER,
      commit : execSync("git rev-parse HEAD || hg id -i --debug | tr -d '+'").toString().trim(),
      build_url : process.env.DRONE_BUILD_URL,
      branch : process.env.DRONE_BRANCH,
      root : process.env.DRONE_BUILD_DIR
    };
  }

};
