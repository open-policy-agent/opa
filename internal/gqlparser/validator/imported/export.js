import fs from "fs";
import Module from "module";
import { testSchema } from "./graphql-js/src/validation/__tests__/harness";
import { printSchema } from "./graphql-js/src/utilities";
import yaml from "js-yaml";

let schemas = [];
function registerSchema(schema) {
  for (let i = 0; i < schemas.length; i++) {
    if (schemas[i] === schema) {
      return i;
    }
  }
  schemas.push(schema);
  return schemas.length - 1;
}

function resultProxy(start, base = {}) {
  const funcWithPath = (path) => {
    const f = () => {};
    f.path = path;
    return f;
  };
  let handler = {
    get: function (obj, prop) {
      if (base[prop]) {
        return base[prop];
      }
      return new Proxy(funcWithPath(`${obj.path}.${prop}`), handler);
    },
  };

  return new Proxy(funcWithPath(start), handler);
}

// replace empty lines with the normal amount of whitespace
// so that yaml correctly preserves the whitespace
function normalizeWs(rawString) {
  const lines = rawString.split(/\r\n|[\n\r]/g);

  let commonIndent = 1000000;
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i];
    if (!line.trim()) {
      continue;
    }

    const indent = line.search(/\S/);
    if (indent < commonIndent) {
      commonIndent = indent;
    }
  }

  for (let i = 1; i < lines.length; i++) {
    if (lines[i].length < commonIndent) {
      lines[i] = " ".repeat(commonIndent);
    }
  }
  return lines.join("\n");
}

const harness = {
  testSchema,

  expectValidationErrorsWithSchema(schema, rule, queryStr) {
    return resultProxy("expectValidationErrorsWithSchema", {
      toDeepEqual(expected) {
        tests.push({
          name: names.slice(1).join("/"),
          rule: rule.name.replace(/Rule$/, ""),
          schema: registerSchema(schema),
          query: normalizeWs(queryStr),
          errors: expected,
        });
      },
    });
  },
  expectValidationErrors(rule, queryStr) {
    return harness.expectValidationErrorsWithSchema(testSchema, rule, queryStr);
  },
  expectSDLValidationErrors(schema, rule, sdlStr) {
    return resultProxy("expectSDLValidationErrors", {
      toDeepEqual(expected) {
        // ignore now...
        // console.warn(rule.name, sdlStr, JSON.stringify(expected, null, 2));
      },
    });
  },
};

let tests = [];
let names = [];
const fakeModules = {
  mocha: {
    describe(name, f) {
      names.push(name);
      f();
      names.pop();
    },
    it(name, f) {
      names.push(name);
      f();
      names.pop();
    },
  },
  chai: {
    expect(it) {
      const expect = {
        get to() {
          return expect;
        },
        get have() {
          return expect;
        },
        get nested() {
          return expect;
        },
        equal(value) {
          // currently ignored, we know all we need to add an assertion here.
        },
        property(path, value) {
          // currently ignored, we know all we need to add an assertion here.
        },
      };

      return expect;
    },
  },
  "./harness": harness,
};

const originalLoader = Module._load;
Module._load = function (request, parent, isMain) {
  return fakeModules[request] || originalLoader(request, parent, isMain);
};

fs.readdirSync("./graphql-js/src/validation/__tests__").forEach((file) => {
  if (!file.endsWith("-test.ts")) {
    return;
  }

  if (file === "validation-test.ts") {
    return;
  }

  require(`./graphql-js/src/validation/__tests__/${file}`);

  let dump = yaml.dump(tests, {
    skipInvalid: true,
    flowLevel: 5,
    noRefs: true,
    lineWidth: 1000,
  });
  fs.writeFileSync(`./spec/${file.replace("-test.ts", ".spec.yml")}`, dump);

  tests = [];
});

let schemaList = schemas.map((s) => printSchema(s));

schemaList[0] += `
# injected becuase upstream spec is missing some types
extend type QueryRoot {
    field: T
    f1: Type
    f2: Type
    f3: Type
}

type Type {
    a: String
    b: String
    c: String
}
type T {
    a: String
    b: String
    c: String
    d: String
    y: String
    deepField: T
    deeperField: T
}`;

let dump = yaml.dump(schemaList, {
  skipInvalid: true,
  flowLevel: 5,
  noRefs: true,
  lineWidth: 1000,
});
fs.writeFileSync("./spec/schemas.yml", dump);
