import assert from "node:assert/strict";
import test from "node:test";

import { buildUrl } from "../src/url.js";

test("joins base and path without duplicate slashes", () => {
  assert.equal(buildUrl("https://api.example.com/", "/v1/users"), "https://api.example.com/v1/users");
  assert.equal(buildUrl("https://api.example.com/root", "v1/users"), "https://api.example.com/root/v1/users");
});

test("preserves existing query parameters and appends new ones", () => {
  assert.equal(
    buildUrl("https://api.example.com/search?lang=en", "/items", { q: "a b", page: 2 }),
    "https://api.example.com/search/items?lang=en&q=a+b&page=2"
  );
});

test("supports arrays and skips nullish query values", () => {
  assert.equal(
    buildUrl("https://api.example.com", "/items", { tag: ["red", "blue"], empty: null, missing: undefined }),
    "https://api.example.com/items?tag=red&tag=blue"
  );
});
