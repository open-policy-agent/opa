/*
    Author: Viktor Semykin <thesame.ml@gmail.com>

    Written for fontello.com project.
*/

'use strict';

var ByteBuffer = require('microbuffer');

/**
 * Offsets in EOT file structure. Refer to EOTPrefix in OpenTypeUtilities.cpp
 */
var EOT_OFFSET = {
  LENGTH:         0,
  FONT_LENGTH:    4,
  VERSION:        8,
  CHARSET:        26,
  MAGIC:          34,
  FONT_PANOSE:    16,
  ITALIC:         27,
  WEIGHT:         28,
  UNICODE_RANGE:  36,
  CODEPAGE_RANGE: 52,
  CHECKSUM_ADJUSTMENT: 60
};

/**
 * Offsets in different SFNT (TTF) structures. See OpenTypeUtilities.cpp
 */
var SFNT_OFFSET = {
    // sfntHeader:
  NUMTABLES:      4,

    // TableDirectoryEntry
  TABLE_TAG:      0,
  TABLE_OFFSET:   8,
  TABLE_LENGTH:   12,

    // OS2Table
  OS2_WEIGHT:         4,
  OS2_FONT_PANOSE:    32,
  OS2_UNICODE_RANGE:  42,
  OS2_FS_SELECTION:   62,
  OS2_CODEPAGE_RANGE: 78,

    // headTable
  HEAD_CHECKSUM_ADJUSTMENT:   8,

    // nameTable
  NAMETABLE_FORMAT:   0,
  NAMETABLE_COUNT:    2,
  NAMETABLE_STRING_OFFSET:    4,

    // nameRecord
  NAME_PLATFORM_ID:   0,
  NAME_ENCODING_ID:   2,
  NAME_LANGUAGE_ID:   4,
  NAME_NAME_ID:       6,
  NAME_LENGTH:        8,
  NAME_OFFSET:        10
};

/**
 * Sizes of structures
 */
var SIZEOF = {
  SFNT_TABLE_ENTRY:   16,
  SFNT_HEADER:        12,
  SFNT_NAMETABLE:          6,
  SFNT_NAMETABLE_ENTRY:    12,
  EOT_PREFIX: 82
};

/**
 * Magic numbers
 */
var MAGIC = {
  EOT_VERSION:    0x00020001,
  EOT_MAGIC:      0x504c,
  EOT_CHARSET:    1,
  LANGUAGE_ENGLISH:   0x0409
};

/**
 * Utility function to convert buffer of utf16be chars to buffer of utf16le
 * chars prefixed with length and suffixed with zero
 */
function strbuf(str) {
  var b = new ByteBuffer(str.length + 4);

  b.setUint16 (0, str.length, true);

  for (var i = 0; i < str.length; i += 2) {
    b.setUint16 (i + 2, str.getUint16 (i), true);
  }

  b.setUint16 (b.length - 2, 0, true);

  return b;
}

// Takes TTF font on input and returns ByteBuffer with EOT font
//
// Params:
//
// - arr(Array|Uint8Array)
//
function ttf2eot(arr) {
  var buf = new ByteBuffer(arr);
  var out = new ByteBuffer(SIZEOF.EOT_PREFIX),
      i, j;

  out.fill(0);
  out.setUint32(EOT_OFFSET.FONT_LENGTH, buf.length, true);
  out.setUint32(EOT_OFFSET.VERSION, MAGIC.EOT_VERSION, true);
  out.setUint8(EOT_OFFSET.CHARSET, MAGIC.EOT_CHARSET);
  out.setUint16(EOT_OFFSET.MAGIC, MAGIC.EOT_MAGIC, true);

  var familyName = [],
      subfamilyName = [],
      fullName = [],
      versionString = [];

  var haveOS2 = false,
      haveName = false,
      haveHead = false;

  var numTables = buf.getUint16 (SFNT_OFFSET.NUMTABLES);

  for (i = 0; i < numTables; ++i) {
    var data = new ByteBuffer(buf, SIZEOF.SFNT_HEADER + i * SIZEOF.SFNT_TABLE_ENTRY);
    var tableEntry = {
      tag: data.toString (SFNT_OFFSET.TABLE_TAG, 4),
      offset: data.getUint32 (SFNT_OFFSET.TABLE_OFFSET),
      length: data.getUint32 (SFNT_OFFSET.TABLE_LENGTH)
    };

    var table = new ByteBuffer(buf, tableEntry.offset, tableEntry.length);

    if (tableEntry.tag === 'OS/2') {
      haveOS2 = true;

      for (j = 0; j < 10; ++j) {
        out.setUint8 (EOT_OFFSET.FONT_PANOSE + j, table.getUint8 (SFNT_OFFSET.OS2_FONT_PANOSE + j));
      }

      /*jshint bitwise:false */
      out.setUint8 (EOT_OFFSET.ITALIC, table.getUint16 (SFNT_OFFSET.OS2_FS_SELECTION) & 0x01);
      out.setUint32 (EOT_OFFSET.WEIGHT, table.getUint16 (SFNT_OFFSET.OS2_WEIGHT), true);

      for (j = 0; j < 4; ++j) {
        out.setUint32 (EOT_OFFSET.UNICODE_RANGE + j * 4, table.getUint32 (SFNT_OFFSET.OS2_UNICODE_RANGE + j * 4), true);
      }

      for (j = 0; j < 2; ++j) {
        out.setUint32 (EOT_OFFSET.CODEPAGE_RANGE + j * 4, table.getUint32 (SFNT_OFFSET.OS2_CODEPAGE_RANGE + j * 4), true);
      }

    } else if (tableEntry.tag === 'head') {

      haveHead = true;
      out.setUint32 (EOT_OFFSET.CHECKSUM_ADJUSTMENT, table.getUint32 (SFNT_OFFSET.HEAD_CHECKSUM_ADJUSTMENT), true);

    } else if (tableEntry.tag === 'name') {

      haveName = true;

      var nameTable = {
        format: table.getUint16 (SFNT_OFFSET.NAMETABLE_FORMAT),
        count: table.getUint16 (SFNT_OFFSET.NAMETABLE_COUNT),
        stringOffset: table.getUint16 (SFNT_OFFSET.NAMETABLE_STRING_OFFSET)
      };

      for (j = 0; j < nameTable.count; ++j) {
        var nameRecord = new ByteBuffer(table, SIZEOF.SFNT_NAMETABLE + j * SIZEOF.SFNT_NAMETABLE_ENTRY);
        var name = {
          platformID: nameRecord.getUint16 (SFNT_OFFSET.NAME_PLATFORM_ID),
          encodingID: nameRecord.getUint16 (SFNT_OFFSET.NAME_ENCODING_ID),
          languageID: nameRecord.getUint16 (SFNT_OFFSET.NAME_LANGUAGE_ID),
          nameID: nameRecord.getUint16 (SFNT_OFFSET.NAME_NAME_ID),
          length: nameRecord.getUint16 (SFNT_OFFSET.NAME_LENGTH),
          offset: nameRecord.getUint16 (SFNT_OFFSET.NAME_OFFSET)
        };

        if (name.platformID === 3 && name.encodingID === 1 && name.languageID === MAGIC.LANGUAGE_ENGLISH) {
          var s = strbuf (new ByteBuffer(table, nameTable.stringOffset + name.offset, name.length));

          switch (name.nameID) {
            case 1:
              familyName = s;
              break;
            case 2:
              subfamilyName = s;
              break;
            case 4:
              fullName = s;
              break;
            case 5:
              versionString = s;
              break;
          }
        }
      }
    }
    if (haveOS2 && haveName && haveHead) { break; }
  }

  if (!(haveOS2 && haveName && haveHead)) {
    throw new Error ('Required section not found');
  }

  // Calculate final length
  var len =
   out.length +
   familyName.length +
   subfamilyName.length +
   versionString.length +
   fullName.length +
   2 +
   buf.length;

  // Create final buffer with the the same array type as input one.
  var eot = new ByteBuffer(len);

  eot.writeBytes(out.buffer);
  eot.writeBytes(familyName.buffer);
  eot.writeBytes(subfamilyName.buffer);
  eot.writeBytes(versionString.buffer);
  eot.writeBytes(fullName.buffer);
  eot.writeBytes([ 0, 0 ]);
  eot.writeBytes(buf.buffer);

  eot.setUint32(EOT_OFFSET.LENGTH, len, true); // Calculate overall length

  return eot;
}

module.exports = ttf2eot;
