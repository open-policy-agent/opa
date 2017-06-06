var env = require('../'),
  expect = require('chai').expect;

describe('gulp-env', function() {
  it('should exist', function() {
    expect(env).to.exist;
  });

  describe('reads properties from files', function() {
    afterEach(function() {
      delete process.env.STARK;
      delete process.env.BARATHEON;
      delete process.env.LANNISTER;
    });

    it('should add process.env vars from a local module', function() {
      expect(process.env.STARK).not.to.exist
      expect(process.env.BARATHEON).not.to.exist
      expect(process.env.LANNISTER).not.to.exist

      env({file: "test/mock-env-module"})

      expect(process.env.STARK).to.equal("direwolf");
      expect(process.env.BARATHEON).to.equal("stag");
      expect(process.env.LANNISTER).to.equal("lion");
    });

    it('should add process.env vars from a local json file', function() {
      expect(process.env.STARK).not.to.exist
      expect(process.env.BARATHEON).not.to.exist
      expect(process.env.LANNISTER).not.to.exist

      env({file: "test/mock-env-json.json"})

      expect(process.env.STARK).to.equal("direwolf");
      expect(process.env.BARATHEON).to.equal("stag");
      expect(process.env.LANNISTER).to.equal("lion");
    });
  });

  describe('reads vars from vars object', function(){
    afterEach(function() {
      delete process.env.NED;
      delete process.env.ROBERT;
      delete process.env.TYWIN;
    });

    it('should add process.env vars from vars object', function() {
      expect(process.env.NED).not.to.exist
      expect(process.env.ROBERT).not.to.exist
      expect(process.env.TYWIN).not.to.exist

      env({vars: {
        NED: true,
        ROBERT: 'fat',
        TYWIN: 9001
      }})

      expect(process.env.NED).to.equal('true');
      expect(process.env.ROBERT).to.equal('fat');
      expect(process.env.TYWIN).to.equal('9001');
    });
  });

  describe('reads properties from files and vars object', function() {
    afterEach(function() {
      delete process.env.STARK;
      delete process.env.BARATHEON;
      delete process.env.LANNISTER;
    });

    it('should overwrite files with inline-vars by default', function() {
      expect(process.env.STARK).not.to.exist

      env({
        file: "test/mock-env-json.json",
        vars: {
          STARK: "wolfenstein"
        }
      });

      expect(process.env.STARK).to.equal('wolfenstein')
      expect(process.env.BARATHEON).to.equal('stag')
      expect(process.env.LANNISTER).to.equal('lion')
    });
  });

})
