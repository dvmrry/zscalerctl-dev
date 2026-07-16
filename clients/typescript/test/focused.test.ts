import { test } from "node:test";
import assert from "node:assert/strict";

import {
  decodeClientFrame,
  decodeCanonicalBase64,
  encodeClientFrame,
  encodeJson,
  encodeOrderedJson,
  encodeServerFrame,
  EngineClientError,
  errorKind,
  ERROR_KINDS,
  NdjsonFrameSplitter,
  NdjsonStreamReader,
  orderedJson,
  parseJsonObject,
  WireNumber,
  spawnEngine,
} from "../src/index.ts";

const encoder = new TextEncoder();
const decoder = new TextDecoder();

test("rejects decoded duplicate keys", () => {
  assert.throws(
    () => decodeClientFrame(encoder.encode('{"type":"cancel","id":1,"\\u0069d":2}')),
    (error: unknown) => errorKind(error) === ERROR_KINDS.DUPLICATE_KEY,
  );
});

test("preserves exact dynamic number lexemes and freezes WireNumber", () => {
  const object = parseJsonObject(encoder.encode('{"large":9007199254740993,"decimal":1.2300e+02}'));
  assert.ok(object.large instanceof WireNumber);
  assert.ok(object.decimal instanceof WireNumber);
  assert.equal(object.large.lexeme, "9007199254740993");
  assert.equal(object.decimal.lexeme, "1.2300e+02");
  assert.equal(Object.isFrozen(object.large), true);
  assert.equal(Object.isFrozen(object.decimal), true);
});

test("handles arbitrary byte splits across a UTF-8 rune", () => {
  const input = encoder.encode('{"x":"☃"}\n');
  const splitter = new NdjsonFrameSplitter(1024);
  const frames: Uint8Array[] = [];
  for (const byte of input) {
    frames.push(...splitter.push(Uint8Array.of(byte)));
  }
  splitter.finish();
  assert.deepEqual(frames.map((frame) => decoder.decode(frame)), ['{"x":"☃"}']);
});

test("rejects depth and size bounds before typed decoding", () => {
  const tooDeep = encoder.encode(`{"x":${"[".repeat(8)}0${"]".repeat(8)}}`);
  assert.throws(
    () => parseJsonObject(tooDeep, { maximumDepth: 8, maximumBytes: 1024 }),
    (error: unknown) => errorKind(error) === ERROR_KINDS.JSON_DEPTH,
  );
  assert.throws(
    () => parseJsonObject(encoder.encode('{"x":"0123456789"}'), { maximumBytes: 8 }),
    (error: unknown) => errorKind(error) === ERROR_KINDS.FRAME_TOO_LARGE,
  );
});

test("rejects empty and oversized NDJSON frames", () => {
  assert.throws(
    () => new NdjsonFrameSplitter(16).push(encoder.encode("\n")),
    (error: unknown) => errorKind(error) === ERROR_KINDS.EMPTY_FRAME,
  );
  assert.throws(
    () => new NdjsonFrameSplitter(4).push(encoder.encode("12345")),
    (error: unknown) => errorKind(error) === ERROR_KINDS.FRAME_TOO_LARGE,
  );
});

test("canonicalizes a mathematically integral structural number", () => {
  const frame = decodeClientFrame(encoder.encode('{"type":"cancel","id":0.01e2}'));
  assert.equal(decoder.decode(encodeClientFrame(frame)), '{"type":"cancel","id":1}');
});

test("typed request encoding rejects a wrong runtime discriminant", () => {
  assert.throws(
    () => encodeClientFrame({
      type: "not-request",
      id: 1,
      capability: "engine.manifest",
      operation: "manifest",
    } as never),
    (error: unknown) => errorKind(error) === ERROR_KINDS.INVALID_FRAME,
  );
});

test("rejects cyclic values in the public JSON encoder", () => {
  const cyclic: Record<string, unknown> = {};
  cyclic.self = cyclic;
  assert.throws(
    () => encodeJson(cyclic as never),
    (error: unknown) => errorKind(error) === ERROR_KINDS.INVALID_FRAME,
  );
});

test("rejects cyclic dynamic records in typed server frames", () => {
  const record: Record<string, unknown> = {};
  record.self = record;
  assert.throws(
    () => encodeServerFrame({
      type: "item",
      id: 1,
      seq: 1,
      kind: "projected_record",
      item: { product: "zia", resource: "locations", record },
    } as never),
    (error: unknown) => errorKind(error) === ERROR_KINDS.INVALID_FRAME,
  );
});

test("rejects duplicate keys in ordered JSON input", () => {
  assert.throws(
    () => encodeOrderedJson([["x", "first"], ["x", "second"]]),
    (error: unknown) => errorKind(error) === ERROR_KINDS.DUPLICATE_KEY,
  );
});

test("ordered JSON snapshots and freezes validated entry tuples", () => {
  const first: [string, string] = ["x", "one"];
  const second: [string, string] = ["y", "two"];
  const object = orderedJson([first, second]);
  first[0] = "y";
  assert.equal(Object.isFrozen(object.entries[0]), true);
  assert.equal(decoder.decode(encodeJson(object as never)), '{"x":"one","y":"two"}');
});

test("strictly decodes fragment base64 at the three chunk boundaries", () => {
  for (const length of [524_287, 524_288]) {
    const source = new Uint8Array(length);
    source[length - 1] = 0xff;
    const encoded = Buffer.from(source).toString("base64");
    assert.deepEqual(decodeCanonicalBase64(encoded), source);
  }
  const oversized = Buffer.from(new Uint8Array(524_289)).toString("base64");
  assert.throws(
    () => decodeCanonicalBase64(oversized),
    (error: unknown) => errorKind(error) === ERROR_KINDS.INVALID_FRAME,
  );
  assert.throws(
    () => decodeCanonicalBase64("AB=="),
    (error: unknown) => errorKind(error) === ERROR_KINDS.INVALID_FRAME,
  );
});

test("stream framing preserves coalesced bytes across a limit change", async () => {
  const source = (async function* (): AsyncGenerator<Uint8Array> {
    yield encoder.encode("{}\n123456789\n");
  })();
  const reader = new NdjsonStreamReader(source);
  assert.equal(decoder.decode(await reader.readFrame(2) as Uint8Array), "{}");
  assert.equal(decoder.decode(await reader.readFrame(9) as Uint8Array), "123456789");
  assert.equal(await reader.readFrame(9), null);
});

test("process adapter refuses PATH-resolved executables", async () => {
  await assert.rejects(
    spawnEngine({ executable: "zscalerctl-engine" }),
    (error: unknown) => error instanceof EngineClientError && error.kind === "request",
  );
});
