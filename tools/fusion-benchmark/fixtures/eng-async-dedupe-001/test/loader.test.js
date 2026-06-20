import assert from "node:assert/strict";
import test from "node:test";

import { CoalescingLoader } from "../src/loader.js";

test("coalesces concurrent loads for the same key", async () => {
  let calls = 0;
  let release;
  const gate = new Promise((resolve) => {
    release = resolve;
  });
  const loader = new CoalescingLoader(async (key) => {
    calls += 1;
    await gate;
    return `value:${key}`;
  });

  const a = loader.load("user:1");
  const b = loader.load("user:1");
  release();

  assert.equal(await a, "value:user:1");
  assert.equal(await b, "value:user:1");
  assert.equal(calls, 1);
});

test("caches successful values", async () => {
  let calls = 0;
  const loader = new CoalescingLoader(async () => {
    calls += 1;
    return "ok";
  });

  assert.equal(await loader.load("k"), "ok");
  assert.equal(await loader.load("k"), "ok");
  assert.equal(calls, 1);
});

test("does not cache rejected fetches", async () => {
  let calls = 0;
  const loader = new CoalescingLoader(async () => {
    calls += 1;
    if (calls === 1) throw new Error("temporary");
    return "recovered";
  });

  await assert.rejects(loader.load("k"), /temporary/);
  assert.equal(await loader.load("k"), "recovered");
  assert.equal(calls, 2);
});
