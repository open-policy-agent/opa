/*
    Author: Viktor Semykin <thesame.ml@gmail.com>

    Written for fontello.com project.
*/

'use strict';


var ByteBuffer = require('microbuffer');
var deflate = require('pako/lib/deflate.js').deflate;


function ulong(t) {
  /*jshint bitwise:false*/
  t &= 0xffffffff;
  if (t < 0) {
    t += 0x100000000;
  }
  return t;
}

function longAlign(n) {
  /*jshint bitwise:false*/
  return (n + 3) & ~3;
}

function calc_checksum(buf) {
  var sum = 0;
  var nlongs = buf.length / 4;

  for (var i = 0; i < nlongs; ++i) {
    var t = buf.getUint32(i * 4);

    sum = ulong(sum + t);
  }
  return sum;
}

var WOFF_OFFSET = {
  MAGIC: 0,
  FLAVOR: 4,
  SIZE: 8,
  NUM_TABLES: 12,
  RESERVED: 14,
  SFNT_SIZE: 16,
  VERSION_MAJ: 20,
  VERSION_MIN: 22,
  META_OFFSET: 24,
  META_LENGTH: 28,
  META_ORIG_LENGTH: 32,
  PRIV_OFFSET: 36,
  PRIV_LENGTH: 40
};

var WOFF_ENTRY_OFFSET = {
  TAG: 0,
  OFFSET: 4,
  COMPR_LENGTH: 8,
  LENGTH: 12,
  CHECKSUM: 16
};

var SFNT_OFFSET = {
  TAG: 0,
  CHECKSUM: 4,
  OFFSET: 8,
  LENGTH: 12
};

var SFNT_ENTRY_OFFSET = {
  FLAVOR: 0,
  VERSION_MAJ: 4,
  VERSION_MIN: 6,
  CHECKSUM_ADJUSTMENT: 8
};

var MAGIC = {
  WOFF: 0x774F4646,
  CHECKSUM_ADJUSTMENT: 0xB1B0AFBA
};

var SIZEOF = {
  WOFF_HEADER: 44,
  WOFF_ENTRY: 20,
  SFNT_HEADER: 12,
  SFNT_TABLE_ENTRY: 16
};

function woffAppendMetadata(src, metadata) {

  var zdata =  deflate(metadata);

  src.setUint32(WOFF_OFFSET.SIZE, src.length + zdata.length);
  src.setUint32(WOFF_OFFSET.META_OFFSET, src.length);
  src.setUint32(WOFF_OFFSET.META_LENGTH, zdata.length);
  src.setUint32(WOFF_OFFSET.META_ORIG_LENGTH, metadata.length);

  //concatenate src and zdata
  var buf = new ByteBuffer(src.length + zdata.length);

  buf.writeBytes(src.toArray());
  buf.writeBytes(zdata);

  return buf;
}

function ttf2woff(arr, options) {
  var buf = new ByteBuffer(arr);

  options = options || {};

  var version = {
    maj: 0,
    min: 1
  };
  var numTables = buf.getUint16(4);
  //var sfntVersion = buf.getUint32 (0);
  var flavor = 0x10000;

  var woffHeader = new ByteBuffer(SIZEOF.WOFF_HEADER);

  woffHeader.setUint32(WOFF_OFFSET.MAGIC, MAGIC.WOFF);
  woffHeader.setUint16(WOFF_OFFSET.NUM_TABLES, numTables);
  woffHeader.setUint16(WOFF_OFFSET.RESERVED, 0);
  woffHeader.setUint32(WOFF_OFFSET.SFNT_SIZE, 0);
  woffHeader.setUint32(WOFF_OFFSET.META_OFFSET, 0);
  woffHeader.setUint32(WOFF_OFFSET.META_LENGTH, 0);
  woffHeader.setUint32(WOFF_OFFSET.META_ORIG_LENGTH, 0);
  woffHeader.setUint32(WOFF_OFFSET.PRIV_OFFSET, 0);
  woffHeader.setUint32(WOFF_OFFSET.PRIV_LENGTH, 0);

  var entries = [];

  var i, tableEntry;

  for (i = 0; i < numTables; ++i) {
    var data = new ByteBuffer(buf.buffer, SIZEOF.SFNT_HEADER + i * SIZEOF.SFNT_TABLE_ENTRY);

    tableEntry = {
      Tag: new ByteBuffer(data, SFNT_OFFSET.TAG, 4),
      checkSum: data.getUint32(SFNT_OFFSET.CHECKSUM),
      Offset: data.getUint32(SFNT_OFFSET.OFFSET),
      Length: data.getUint32(SFNT_OFFSET.LENGTH)
    };
    entries.push (tableEntry);
  }
  entries = entries.sort(function (a, b) {
    var aStr = a.Tag.toString();
    var bStr = b.Tag.toString();

    return aStr === bStr ? 0 : aStr < bStr ? -1 : 1;
  });

  var offset = SIZEOF.WOFF_HEADER + numTables * SIZEOF.WOFF_ENTRY;
  var woffSize = offset;
  var sfntSize = SIZEOF.SFNT_HEADER + numTables * SIZEOF.SFNT_TABLE_ENTRY;

  var tableBuf = new ByteBuffer(numTables * SIZEOF.WOFF_ENTRY);

  for (i = 0; i < numTables; ++i) {
    tableEntry = entries[i];

    if (tableEntry.Tag.toString() !== 'head') {
      var algntable = new ByteBuffer(buf.buffer, tableEntry.Offset, longAlign(tableEntry.Length));

      if (calc_checksum(algntable) !== tableEntry.checkSum) {
        throw 'Checksum error in ' + tableEntry.Tag.toString();
      }
    }

    tableBuf.setUint32(i * SIZEOF.WOFF_ENTRY + WOFF_ENTRY_OFFSET.TAG, tableEntry.Tag.getUint32(0));
    tableBuf.setUint32(i * SIZEOF.WOFF_ENTRY + WOFF_ENTRY_OFFSET.LENGTH, tableEntry.Length);
    tableBuf.setUint32(i * SIZEOF.WOFF_ENTRY + WOFF_ENTRY_OFFSET.CHECKSUM, tableEntry.checkSum);
    sfntSize += longAlign(tableEntry.Length);
  }

  var sfntOffset = SIZEOF.SFNT_HEADER + entries.length * SIZEOF.SFNT_TABLE_ENTRY;
  var csum = calc_checksum (new ByteBuffer(buf.buffer, 0, SIZEOF.SFNT_HEADER));

  for (i = 0; i < entries.length; ++i) {
    tableEntry = entries[i];

    var b = new ByteBuffer(SIZEOF.SFNT_TABLE_ENTRY);

    b.setUint32(SFNT_OFFSET.TAG, tableEntry.Tag.getUint32(0));
    b.setUint32(SFNT_OFFSET.CHECKSUM, tableEntry.checkSum);
    b.setUint32(SFNT_OFFSET.OFFSET, sfntOffset);
    b.setUint32(SFNT_OFFSET.LENGTH, tableEntry.Length);
    sfntOffset += longAlign(tableEntry.Length);
    csum += calc_checksum (b);
    csum += tableEntry.checkSum;
  }

  var checksumAdjustment = ulong(MAGIC.CHECKSUM_ADJUSTMENT - csum);

  var len, woffDataChains = [];

  for (i = 0; i < entries.length; ++i) {
    tableEntry = entries[i];

    var sfntData = new ByteBuffer(buf.buffer, tableEntry.Offset, tableEntry.Length);

    if (tableEntry.Tag.toString() === 'head') {
      version.maj = sfntData.getUint16(SFNT_ENTRY_OFFSET.VERSION_MAJ);
      version.min = sfntData.getUint16(SFNT_ENTRY_OFFSET.VERSION_MIN);
      flavor = sfntData.getUint32(SFNT_ENTRY_OFFSET.FLAVOR);
      sfntData.setUint32 (SFNT_ENTRY_OFFSET.CHECKSUM_ADJUSTMENT, checksumAdjustment);
    }

    var res = deflate(sfntData.toArray());

    var compLength;

    // We should use compression only if it really save space (standard requirement).
    // Also, data should be aligned to long (with zeros?).
    compLength = Math.min(res.length, sfntData.length);
    len = longAlign(compLength);

    var woffData = new ByteBuffer(len);

    woffData.fill(0);

    if (res.length >= sfntData.length) {
      woffData.writeBytes(sfntData.toArray());
    } else {
      woffData.writeBytes(res);
    }

    tableBuf.setUint32(i * SIZEOF.WOFF_ENTRY + WOFF_ENTRY_OFFSET.OFFSET, offset);

    offset += woffData.length;
    woffSize += woffData.length;

    tableBuf.setUint32(i * SIZEOF.WOFF_ENTRY + WOFF_ENTRY_OFFSET.COMPR_LENGTH, compLength);

    woffDataChains.push(woffData);
  }

  woffHeader.setUint32(WOFF_OFFSET.SIZE, woffSize);
  woffHeader.setUint32(WOFF_OFFSET.SFNT_SIZE, sfntSize);
  woffHeader.setUint16(WOFF_OFFSET.VERSION_MAJ, version.maj);
  woffHeader.setUint16(WOFF_OFFSET.VERSION_MIN, version.min);
  woffHeader.setUint32(WOFF_OFFSET.FLAVOR, flavor);

  var out = new ByteBuffer(woffSize);

  out.writeBytes(woffHeader.buffer);
  out.writeBytes(tableBuf.buffer);

  for (i = 0; i < woffDataChains.length; i++) {
    out.writeBytes(woffDataChains[i].buffer);
  }

  if (!options.metadata) return out;

  return woffAppendMetadata(out, options.metadata);
}

module.exports = ttf2woff;
