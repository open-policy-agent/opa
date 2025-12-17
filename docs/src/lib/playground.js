function init() {}

function findPrefix(prefix, files) {
  return Object.entries(files)
    .filter(([key]) => key.startsWith(prefix) && key.endsWith(".json"))
    .map((tuple) => JSON.parse(tuple[1]))
    .reduce((o, content) => ({ ...o, ...content }), {});
}

async function exec({ command, files }) {
  if (command === "lint") return lint(files);

  const input = findPrefix("input", files);
  const data = findPrefix("data", files);
  const rego = command != "run" ? command : undefined;
  const req = {
    rego,
    input,
    data,
    rego_modules: Object.fromEntries(
      Object.entries(files)
        .filter(([key]) => key === "" || key.endsWith(".rego"))
        .map(([key, contents]) => [key === "" ? "module.rego" : key, contents]),
    ),
  };
  const resp = await fetch("https://play.openpolicyagent.org/v1/data", {
    method: "POST",
    body: JSON.stringify(req),
  });
  const { ok } = resp;
  if (ok) {
    const body = await resp.json();
    const duration = formatNanosecondTime(body.eval_time);
    if (!body?.result[0]) {
      return {
        ok,
        duration,
        stderr: "undefined",
      };
    }
    return {
      ok,
      duration,
      stdout: JSON.stringify(body.result[0].expressions[0].value, undefined, 2),
    };
  }

  const errorText = await resp.text();
  return {
    ok,
    stderr: errorText
      .replace(/^"|"$/g, "")
      .replace(/\\n/g, "\n")
      .replace(/\\t/g, "\t"),
  };
}

function formatNanosecondTime(nanoseconds) {
  if (nanoseconds < 1000) {
    return `${nanoseconds}ns`;
  } else if (nanoseconds < 1000000) {
    return `${Math.round(nanoseconds / 1000)}µs`;
  } else if (nanoseconds < 1000000000) {
    return `${Math.round(nanoseconds / 1000000)}ms`;
  } else {
    return `${Math.round(nanoseconds / 1000000000)}s`;
  }
}

async function lint(files) {
  const resp = await fetch("https://play.openpolicyagent.org/v1/lint", {
    method: "POST",
    body: JSON.stringify({ rego_module: files[""] }),
  });
  const { ok } = resp;
  if (ok) {
    const body = await resp.json();
    const errorMsg = body.error_message;
    return {
      ok,
      stdout: JSON.stringify(body.report, undefined, 2),
      stderr: errorMsg,
    };
  }
}

// add the engine to the registry
if (typeof window !== `undefined`) {
  window.codapi.engines = {
    ...window.codapi.engines,
    ...{ playground: { init, exec } },
  };
}

export default { init, exec };
