import test from "node:test";
import assert from "node:assert/strict";
import { parseEpisodeNumbers } from "./episodes.ts";

test("exact episode parsing preserves gaps and canonical ordering", () => {
  assert.deepEqual(parseEpisodeNumbers("4, 1, 2"), [1, 2, 4]);
  assert.deepEqual(parseEpisodeNumbers(""), []);
  assert.deepEqual(parseEpisodeNumbers("  "), []);
  assert.deepEqual(parseEpisodeNumbers("100000"), [100000]);
});
test("exact episode parsing rejects ambiguous and excessive selections", () => {
  for (const value of ["0", "-1", "100001", "1,1", "1,01", "1.5", "1e2", "1-3", "1,", ",1", "1,,2", "+1", "x", "9".repeat(71000), Array(10001).fill("1").join(",")]) {
    assert.throws(() => parseEpisodeNumbers(value));
  }
});
