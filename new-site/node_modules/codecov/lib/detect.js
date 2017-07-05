var services = {
  'travis' : require('./services/travis'),
  'circle' : require('./services/circle'),
  'buildkite' : require('./services/buildkite'),
  'codeship' : require('./services/codeship'),
  'drone' : require('./services/drone'),
  'appveyor' : require('./services/appveyor'),
  'wercker' : require('./services/wercker'),
  'jenkins' : require('./services/jenkins'),
  'semaphore' : require('./services/semaphore'),
  'snap' : require('./services/snap'),
  'gitlab' : require('./services/gitlab')
};

var detectProvider = function(){
  var config;
  for (var name in services){
    if (services[name].detect()){
      config = services[name].configuration();
      break;
    }
  }
  if (!config){
    var local = require('./services/localGit');
    config = local.configuration();
    if (!config){
      throw new Error("Unknown CI servie provider. Unable to upload coverage.");
    }
  }
  return config;
};

module.exports = detectProvider;
