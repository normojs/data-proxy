import assert from "node:assert/strict";
import test from "node:test";

import { TtlCache } from "../src/cache.js";

test("returns values before expiration", () => {
  let now = 1000;
  const cache = new TtlCache(() => now);

  cache.set("token", "abc", 50);
  now = 1049;

  assert.equal(cache.get("token"), "abc");
  assert.equal(cache.has("token"), true);
});

test("removes expired values", () => {
  let now = 1000;
  const cache = new TtlCache(() => now);

  cache.set("token", "abc", 50);
  now = 1050;

  assert.equal(cache.get("token"), undefined);
  assert.equal(cache.has("token"), false);
  assert.equal(cache.items.has("token"), false);
});
