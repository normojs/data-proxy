import assert from "node:assert/strict";
import test from "node:test";

import { parseCsv } from "../src/csv.js";

test("parses simple rows", () => {
  assert.deepEqual(parseCsv("a,b\n1,2"), [
    ["a", "b"],
    ["1", "2"]
  ]);
});

test("handles quoted commas, quotes, and newlines", () => {
  assert.deepEqual(parseCsv('name,notes\n"Ada","hello, world"\n"Bob","line 1\nline 2"\n"Q","a ""quote"""'), [
    ["name", "notes"],
    ["Ada", "hello, world"],
    ["Bob", "line 1\nline 2"],
    ["Q", 'a "quote"']
  ]);
});

test("rejects malformed quoted fields", () => {
  assert.throws(() => parseCsv('"unterminated'), /csv/i);
  assert.throws(() => parseCsv('"ok"x'), /csv/i);
});
