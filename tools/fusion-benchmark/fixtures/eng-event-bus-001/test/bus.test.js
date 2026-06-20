import assert from "node:assert/strict";
import test from "node:test";

import { EventBus } from "../src/bus.js";

test("on returns an unsubscribe function", () => {
  const bus = new EventBus();
  const seen = [];
  const unsubscribe = bus.on("msg", (value) => seen.push(value));

  bus.emit("msg", 1);
  unsubscribe();
  bus.emit("msg", 2);

  assert.deepEqual(seen, [1]);
});

test("once handlers are removed after the first emit", () => {
  const bus = new EventBus();
  let count = 0;

  const unsubscribe = bus.once("ready", () => {
    count += 1;
  });
  bus.emit("ready");
  bus.emit("ready");
  unsubscribe();

  assert.equal(count, 1);
});

test("removing a handler during emit does not skip remaining handlers", () => {
  const bus = new EventBus();
  const seen = [];
  let removeFirst;
  removeFirst = bus.on("x", () => {
    seen.push("first");
    removeFirst();
  });
  bus.on("x", () => seen.push("second"));

  bus.emit("x");
  bus.emit("x");

  assert.deepEqual(seen, ["first", "second", "second"]);
});
