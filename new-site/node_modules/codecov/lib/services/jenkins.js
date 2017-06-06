module.exports = {

  detect : function(){
    return !!process.env.JENKINS_URL;
  },

  configuration : function(){
    console.log('    Jenkins CI Detected');
    return {
      service : 'jenkins',
      commit : process.env.ghprbActualCommit || process.env.GIT_COMMIT,
      branch : process.env.ghprbSourceBranch || process.env.GIT_BRANCH,
      build :  process.env.BUILD_NUMBER,
      build_url :  process.env.BUILD_URL,
      root : process.env.WORKSPACE,
      pr : process.env.ghprbPullId
    };
  }

};
