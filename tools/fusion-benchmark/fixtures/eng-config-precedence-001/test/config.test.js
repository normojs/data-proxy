import assert from "node:assert/strict";
import test from "node:test";

import { loadConfig, parseArgs } from "../src/config.js";

test("parses --key value and --key=value forms", () => {
  assert.deepEqual(parseArgs(["--region", "eu", "--config=prod.json", "--verbose"]), {
    region: "eu",
    config: "prod.json",
    verbose: true
  });
});

test("CLI flags override environment and defaults", () => {
  const cfg = loadConfig({
    argv: ["--region", "ap", "--config", "cli.json"],
    env: { APP_REGION: "eu", APP_CONFIG: "env.json" },
    defaults: { region: "us", config: "default.json" }
  });
  assert.equal(cfg.region, "ap");
  assert.equal(cfg.config, "cli.json");
});

test("environment overrides defaults when CLI is absent", () => {
  const cfg = loadConfig({
    argv: [],
    env: { APP_REGION: "eu", APP_VERBOSE: "1" },
    defaults: { region: "us", verbose: false }
  });
  assert.equal(cfg.region, "eu");
  assert.equal(cfg.verbose, true);
});
