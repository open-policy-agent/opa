var execSync = require('child_process').execSync;
if (!execSync) {
  var exec = require('execSync').exec;
  var execSync = function(cmd){
    return exec(cmd).stdout;
  };
}

module.exports = {

  configuration : function(){
    console.log('    No CI Detected. Using git/mercurial');
    var branch = execSync("git rev-parse --abbrev-ref HEAD || hg branch").toString().trim();
    if (branch === 'HEAD') {
      branch = 'master';
    }
    var head = execSync("git rev-parse HEAD || hg id -i --debug | tr -d '+'").toString().trim();
    return {
      commit : head,
      branch : branch
    };
  }

};
