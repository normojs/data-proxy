import assert from "node:assert/strict";
import test from "node:test";

import { LruCache } from "../src/lru.js";

test("get marks entries as recently used", () => {
  const cache = new LruCache(2);
  cache.set("a", 1);
  cache.set("b", 2);
  assert.equal(cache.get("a"), 1);
  cache.set("c", 3);

  assert.equal(cache.has("a"), true);
  assert.equal(cache.has("b"), false);
  assert.equal(cache.has("c"), true);
});

test("updating an existing key does not evict another key", () => {
  const cache = new LruCache(2);
  cache.set("a", 1);
  cache.set("b", 2);
  cache.set("a", 10);

  assert.equal(cache.get("a"), 10);
  assert.equal(cache.has("b"), true);
});

test("capacity zero stores nothing", () => {
  const cache = new LruCache(0);
  cache.set("a", 1);
  assert.equal(cache.has("a"), false);
  assert.equal(cache.get("a"), undefined);
});
