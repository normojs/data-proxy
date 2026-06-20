import assert from "node:assert/strict";
import test from "node:test";

import { retry } from "../src/retry.js";

test("retries failures and returns successful value", async () => {
  const attempts = [];
  const waits = [];

  const value = await retry(
    async (attempt) => {
      attempts.push(attempt);
      if (attempt < 2) throw new Error(`fail ${attempt}`);
      return "ok";
    },
    { retries: 3, wait: async (attempt) => waits.push(attempt) }
  );

  assert.equal(value, "ok");
  assert.deepEqual(attempts, [0, 1, 2]);
  assert.deepEqual(waits, [0, 1]);
});

test("retries means extra attempts after the first try", async () => {
  let calls = 0;
  await assert.rejects(
    retry(
      async () => {
        calls += 1;
        throw new Error("still down");
      },
      { retries: 2 }
    ),
    /still down/
  );
  assert.equal(calls, 3);
});

test("does not wait or retry when shouldRetry rejects the error", async () => {
  let waits = 0;
  await assert.rejects(
    retry(
      async () => {
        throw new TypeError("bad input");
      },
      {
        retries: 5,
        shouldRetry: (err) => !(err instanceof TypeError),
        wait: async () => {
          waits += 1;
        }
      }
    ),
    /bad input/
  );
  assert.equal(waits, 0);
});
