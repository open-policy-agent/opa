// http://doc.gitlab.com/ci/examples/README.html#environmental-variables
// https://gitlab.com/gitlab-org/gitlab-ci-runner/blob/master/lib/build.rb#L96

module.exports = {

  detect : function(){
    return process.env.CI_SERVER_NAME == 'GitLab CI';
  },

  configuration : function(){
    console.log('    Gitlab CI Detected');
    return {
      service : 'gitlab',
      build :  process.env.CI_BUILD_ID,
      commit : process.env.CI_BUILD_REF,
      branch : process.env.CI_BUILD_REF_NAME,
      root : process.env.CI_PROJECT_DIR,
      slug : process.env.CI_BUILD_REPO.split('/').slice(3, 5).join('/').replace('.git', '')
    };
  }

};
