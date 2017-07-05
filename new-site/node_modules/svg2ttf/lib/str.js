'use strict';

function Str(str) {
  if (!(this instanceof Str)) {
    return new Str(str);
  }

  this.str = str;

  this.toUTF8Bytes = function () {

    var byteArray = [];
    for (var i = 0; i < str.length; i++) {
      if (str.charCodeAt(i) <= 0x7F) {
        byteArray.push(str.charCodeAt(i));
      } else {
        var h = encodeURIComponent(str.charAt(i)).substr(1).split('%');
        for (var j = 0; j < h.length; j++) {
          byteArray.push(parseInt(h[j], 16));
        }
      }
    }
    return byteArray;
  };

  this.toUCS2Bytes = function () {
    // Code is taken here:
    // http://stackoverflow.com/questions/6226189/how-to-convert-a-string-to-bytearray
    var byteArray = [];
    var ch;

    for (var i = 0; i < str.length; ++i) {
      ch = str.charCodeAt(i);  // get char
      /*jshint bitwise:false*/
      byteArray.push(ch >> 8);
      byteArray.push(ch & 0xFF);
      /*jshint bitwise:true*/
    }
    return byteArray;
  };
}

module.exports = Str;