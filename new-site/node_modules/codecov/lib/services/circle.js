module.exports = {

  detect : function(){
    return !!process.env.CIRCLECI;
  },

  configuration : function(){
    console.log('    Circle CI Detected');
    return {
      service : 'circleci',
      build : process.env.CIRCLE_BUILD_NUM + '.' + process.env.CIRCLE_NODE_INDEX,
      job : process.env.CIRCLE_BUILD_NUM + '.' + process.env.CIRCLE_NODE_INDEX,
      commit : process.env.CIRCLE_SHA1,
      branch : process.env.CIRCLE_BRANCH,
      pr: process.env.CIRCLE_PR_NUMBER,
      slug : process.env.CIRCLE_PROJECT_USERNAME + '/' + process.env.CIRCLE_PROJECT_REPONAME,
    };
  }

};
