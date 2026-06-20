import assert from "node:assert/strict";
import test from "node:test";

import { TokenBucket } from "../src/limiter.js";

test("allows up to capacity then denies until refilled", () => {
  let now = 0;
  const bucket = new TokenBucket({ capacity: 3, refillPerMs: 0.1, now: () => now });

  assert.equal(bucket.allow(), true);
  assert.equal(bucket.allow(), true);
  assert.equal(bucket.allow(), true);
  assert.equal(bucket.allow(), false);

  now = 10;
  assert.equal(bucket.allow(), true);
  assert.equal(bucket.allow(), false);
});

test("caps refilled tokens at capacity", () => {
  let now = 0;
  const bucket = new TokenBucket({ capacity: 5, refillPerMs: 10, now: () => now });
  bucket.allow(4);
  now = 1000;

  assert.equal(bucket.allow(5), true);
  assert.equal(bucket.allow(1), false);
});

test("rejects invalid costs without mutating state", () => {
  let now = 0;
  const bucket = new TokenBucket({ capacity: 2, refillPerMs: 1, now: () => now });

  assert.throws(() => bucket.allow(0), /cost/i);
  assert.throws(() => bucket.allow(-1), /cost/i);
  assert.equal(bucket.allow(2), true);
  assert.equal(bucket.allow(1), false);
});
