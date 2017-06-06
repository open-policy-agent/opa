
module.exports = {

  detect : function(){
    return !!process.env.SEMAPHORE;
  },

  configuration : function(){
    console.log('    Semaphore CI Detected');
    return {
      service : 'semaphore',
      build : process.env.SEMAPHORE_BUILD_NUMBER + '.' + process.env.SEMAPHORE_CURRENT_THREAD,
      commit : process.env.REVISION,
      branch : process.env.BRANCH_NAME,
      slug : process.env.SEMAPHORE_REPO_SLUG
    };
  }

};
