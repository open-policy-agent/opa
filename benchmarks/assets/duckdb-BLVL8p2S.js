import { _ as __vitePreload } from "./index-P_Dt1GqB.js";
import * as duckdb from "https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/+esm";
const bundles = {
  mvp: {
    mainModule: "https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/dist/duckdb-mvp.wasm",
    mainWorker: "https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/dist/duckdb-browser-mvp.worker.js"
  },
  eh: {
    mainModule: "https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/dist/duckdb-eh.wasm",
    mainWorker: "https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/dist/duckdb-browser-eh.worker.js"
  }
};
const bundle = duckdb.selectBundle(bundles);
const logger = new duckdb.ConsoleLogger(duckdb.LogLevel.WARNING);
class DuckDBClient {
  constructor(db) {
    Object.defineProperties(this, {
      _db: { value: db }
    });
  }
  async queryStream(query, params) {
    const connection = await this._db.connect();
    let reader, batch;
    try {
      if (params?.length > 0) {
        const statement = await connection.prepare(query);
        reader = await statement.send(...params);
      } else {
        reader = await connection.send(query);
      }
      batch = await reader.next();
      if (batch.done)
        throw new Error("missing first batch");
    } catch (error) {
      await connection.close();
      throw error;
    }
    return {
      schema: batch.value.schema,
      async *readRows() {
        try {
          while (!batch.done) {
            yield batch.value.toArray();
            batch = await reader.next();
          }
        } finally {
          await connection.close();
        }
      }
    };
  }
  async query(query, params) {
    const connection = await this._db.connect();
    let table;
    try {
      if (params?.length > 0) {
        const statement = await connection.prepare(query);
        table = await statement.query(...params);
      } else {
        table = await connection.query(query);
      }
    } finally {
      await connection.close();
    }
    return table;
  }
  async queryRow(query, params) {
    const result = await this.queryStream(query, params);
    const reader = result.readRows();
    try {
      const { done, value } = await reader.next();
      return done || !value.length ? null : value[0];
    } finally {
      await reader.return();
    }
  }
  async sql(strings, ...args) {
    return await this.query(strings.join("?"), args);
  }
  queryTag(strings, ...params) {
    return [strings.join("?"), params];
  }
  escape(name) {
    return `"${name}"`;
  }
  async describeTables() {
    return Array.from(await this.query("SHOW TABLES"), ({ name }) => ({ name }));
  }
  async describeColumns(options = {}) {
    return Array.from(await this.query(`DESCRIBE ${this.escape(options.table)}`), ({ column_name, column_type, null: nullable }) => ({
      name: column_name,
      type: getDuckDBType(column_type),
      nullable: nullable !== "NO",
      databaseType: column_type
    }));
  }
  static async of(sources = {}, config = {}) {
    const db = await createDuckDB();
    if (config.query?.castDecimalToDouble === void 0) {
      config = { ...config, query: { ...config.query, castDecimalToDouble: true } };
    }
    if (config.query?.castTimestampToDate === void 0) {
      config = { ...config, query: { ...config.query, castTimestampToDate: true } };
    }
    if (config.query?.castBigIntToDouble === void 0) {
      config = { ...config, query: { ...config.query, castBigIntToDouble: true } };
    }
    await db.open(config);
    await Promise.all(Object.entries(sources).map(([name, source]) => insertSource(db, name, source)));
    return new DuckDBClient(db);
  }
  static sql() {
    return this.of.apply(this, arguments).then((db) => db.sql.bind(db));
  }
}
Object.defineProperty(DuckDBClient.prototype, "dialect", { value: "duckdb" });
async function insertSource(database, name, source) {
  source = await source;
  if (isFileAttachment(source))
    return insertFile(database, name, source);
  if (isArrowTable(source))
    return insertArrowTable(database, name, source);
  if (Array.isArray(source))
    return insertArray(database, name, source);
  if (isArqueroTable(source))
    return insertArqueroTable(database, name, source);
  if (typeof source === "string")
    return insertUrl(database, name, source);
  if (source && typeof source === "object") {
    if ("data" in source) {
      const { data, ...options } = source;
      if (isArrowTable(data))
        return insertArrowTable(database, name, data, options);
      return insertArray(database, name, data, options);
    }
    if ("file" in source) {
      const { file, ...options } = source;
      return insertFile(database, name, file, options);
    }
  }
  throw new Error(`invalid source: ${source}`);
}
async function insertUrl(database, name, url) {
  const connection = await database.connect();
  try {
    await connection.query(`CREATE VIEW '${name}' AS FROM '${url}'`);
  } finally {
    await connection.close();
  }
}
async function insertFile(database, name, file, options) {
  const url = await file.url();
  if (url.startsWith("blob:")) {
    const buffer = await file.arrayBuffer();
    await database.registerFileBuffer(file.name, new Uint8Array(buffer));
  } else {
    await database.registerFileURL(file.name, new URL(url, location).href, 4);
  }
  const connection = await database.connect();
  try {
    switch (file.mimeType) {
      case "text/csv":
      case "text/tab-separated-values": {
        return await connection.insertCSVFromPath(file.name, {
          name,
          schema: "main",
          ...options
        }).catch(async (error) => {
          if (error.toString().includes("Could not convert")) {
            return await insertUntypedCSV(connection, file, name);
          }
          throw error;
        });
      }
      case "application/json":
        return await connection.insertJSONFromPath(file.name, {
          name,
          schema: "main",
          ...options
        });
      default:
        if (/\.arrow$/i.test(file.name)) {
          const buffer = new Uint8Array(await file.arrayBuffer());
          return await connection.insertArrowFromIPCStream(buffer, {
            name,
            schema: "main",
            ...options
          });
        }
        if (/\.parquet$/i.test(file.name)) {
          const table = file.size < 5e7 ? "TABLE" : "VIEW";
          return await connection.query(`CREATE ${table} '${name}' AS SELECT * FROM parquet_scan('${file.name}')`);
        }
        if (/\.(db|ddb|duckdb)$/i.test(file.name)) {
          return await connection.query(`ATTACH '${file.name}' AS ${name} (READ_ONLY)`);
        }
        throw new Error(`unknown file type: ${file.mimeType}`);
    }
  } finally {
    await connection.close();
  }
}
async function insertUntypedCSV(connection, file, name) {
  const statement = await connection.prepare(`CREATE TABLE '${name}' AS SELECT * FROM read_csv_auto(?, ALL_VARCHAR=TRUE)`);
  return await statement.send(file.name);
}
async function insertArrowTable(database, name, table, options) {
  const connection = await database.connect();
  try {
    await connection.insertArrowTable(table, {
      name,
      schema: "main",
      ...options
    });
  } finally {
    await connection.close();
  }
}
async function insertArqueroTable(database, name, source) {
  const arrow = await __vitePreload(() => import("https://cdn.jsdelivr.net/npm/apache-arrow@17.0.0/+esm"), true ? [] : void 0);
  const table = arrow.tableFromIPC(source.toArrowBuffer());
  return await insertArrowTable(database, name, table);
}
async function insertArray(database, name, array, options) {
  const arrow = await __vitePreload(() => import("https://cdn.jsdelivr.net/npm/apache-arrow@17.0.0/+esm"), true ? [] : void 0);
  const table = arrow.tableFromJSON(array);
  return await insertArrowTable(database, name, table, options);
}
async function createDuckDB() {
  const { mainWorker, mainModule } = await bundle;
  const worker = await duckdb.createWorker(mainWorker);
  const db = new duckdb.AsyncDuckDB(logger, worker);
  await db.instantiate(mainModule);
  return db;
}
function getDuckDBType(type) {
  switch (type) {
    case "BIGINT":
    case "HUGEINT":
    case "UBIGINT":
      return "bigint";
    case "DOUBLE":
    case "REAL":
    case "FLOAT":
      return "number";
    case "INTEGER":
    case "SMALLINT":
    case "TINYINT":
    case "USMALLINT":
    case "UINTEGER":
    case "UTINYINT":
      return "integer";
    case "BOOLEAN":
      return "boolean";
    case "DATE":
    case "TIMESTAMP":
    case "TIMESTAMP WITH TIME ZONE":
      return "date";
    case "VARCHAR":
    case "UUID":
      return "string";
    // case "BLOB":
    // case "INTERVAL":
    // case "TIME":
    default:
      if (/^DECIMAL\(/.test(type))
        return "integer";
      return "other";
  }
}
function isFileAttachment(value) {
  return value && typeof value.name === "string" && typeof value.url === "function" && typeof value.arrayBuffer === "function";
}
function isArqueroTable(value) {
  return value && typeof value.toArrowBuffer === "function";
}
function isArrowTable(value) {
  return value && typeof value.getChild === "function" && typeof value.toArray === "function" && value.schema && Array.isArray(value.schema.fields);
}
export {
  DuckDBClient
};
