export function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) continue;
    const key = arg.slice(2);
    out[key] = argv[i + 1];
    i += 1;
  }
  return out;
}

export function loadConfig({ argv = [], env = {}, defaults = {} } = {}) {
  const args = parseArgs(argv);
  return {
    region: env.APP_REGION || args.region || defaults.region || "us",
    config: env.APP_CONFIG || args.config || defaults.config || "app.json",
    verbose: Boolean(env.APP_VERBOSE || args.verbose || defaults.verbose)
  };
}
