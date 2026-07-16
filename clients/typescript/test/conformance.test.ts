import { readFile } from "node:fs/promises";
import { test } from "node:test";
import assert from "node:assert/strict";

import {
  decodeBootstrapClientFrame,
  decodeBootstrapServerFrame,
  decodeClientFrame,
  decodeServerFrame,
  encodeBootstrapClientFrame,
  encodeBootstrapServerFrame,
  encodeClientFrame,
  encodeServerFrame,
  errorKind,
  NdjsonFrameSplitter,
  parseJsonValue,
  WireNumber,
} from "../src/index.ts";

type FixtureObject = { readonly [key: string]: FixtureValue };
type FixtureValue = null | boolean | number | string | WireNumber | FixtureValue[] | FixtureObject;

const encoder = new TextEncoder();
const decoder = new TextDecoder();
const corpusRoot = new URL("../../../internal/enginewire/testdata/conformance/", import.meta.url);

function fixtureObject(value: FixtureValue): FixtureObject {
  assert.equal(typeof value, "object");
  assert.notEqual(value, null);
  assert.equal(Array.isArray(value), false);
  return value as FixtureObject;
}

function fixtureString(value: FixtureValue | undefined): string {
  assert.equal(typeof value, "string");
  return value as string;
}

function fixtureArray(value: FixtureValue | undefined): FixtureValue[] {
  assert.equal(Array.isArray(value), true);
  return value as FixtureValue[];
}

function fixtureNumber(value: FixtureValue | undefined): number {
  if (typeof value === "number") return value;
  assert.ok(value instanceof WireNumber);
  return Number(value.lexeme);
}

function decodeBase64(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

async function loadFixture(name: string): Promise<FixtureObject> {
  const text = await readFile(new URL(name, corpusRoot), "utf8");
  return JSON.parse(text) as FixtureObject;
}

function codecDecoder(name: string): (data: Uint8Array) => unknown {
  switch (name) {
    case "bootstrap_client": return decodeBootstrapClientFrame;
    case "bootstrap_server": return decodeBootstrapServerFrame;
    case "v1_client": return decodeClientFrame;
    case "v1_server": return decodeServerFrame;
    default: throw new Error(`unknown fixture codec ${name}`);
  }
}

function codecEncoder(name: string): (frame: never) => Uint8Array {
  switch (name) {
    case "bootstrap_client": return encodeBootstrapClientFrame as (frame: never) => Uint8Array;
    case "bootstrap_server": return encodeBootstrapServerFrame as (frame: never) => Uint8Array;
    case "v1_client": return encodeClientFrame as (frame: never) => Uint8Array;
    case "v1_server": return encodeServerFrame as (frame: never) => Uint8Array;
    default: throw new Error(`unknown fixture codec ${name}`);
  }
}

test("shared Go codec corpus", async () => {
  const corpus = await loadFixture("codec-v1.json");
  assert.equal(fixtureString(corpus.version), "zscalerctl.engine.stdio.codec-conformance.v1");
  for (const rawCase of fixtureArray(corpus.cases)) {
    const testCase = fixtureObject(rawCase);
    const name = fixtureString(testCase.name);
    const codec = fixtureString(testCase.codec);
    const input = testCase.input_base64 === undefined
      ? encoder.encode(fixtureString(testCase.input))
      : decodeBase64(fixtureString(testCase.input_base64));
    const decode = codecDecoder(codec);
    const encode = codecEncoder(codec);
    const expectedError = testCase.error === undefined ? undefined : fixtureString(testCase.error);
    let frame: unknown;
    let caught: unknown;
    try {
      frame = decode(input);
    } catch (error) {
      caught = error;
    }
    if (expectedError !== undefined) {
      assert.equal(errorKind(caught), expectedError, name);
      continue;
    }
    assert.equal(caught, undefined, name);
    const output = decoder.decode(encode(frame as never));
    assert.equal(output, fixtureString(testCase.output), name);
    assert.equal(fixtureString(testCase.frame_type), fixtureString(fixtureObject(parseJsonValue(encoder.encode(output))).type), name);
  }
});

test("shared Go framing corpus", async () => {
  const corpus = await loadFixture("framing-v1.json");
  assert.equal(fixtureString(corpus.version), "zscalerctl.engine.stdio.framing-conformance.v1");
  for (const rawCase of fixtureArray(corpus.cases)) {
    const testCase = fixtureObject(rawCase);
    const splitter = new NdjsonFrameSplitter(fixtureNumber(testCase.maximum_bytes));
    const frames: Uint8Array[] = [];
    let caught: unknown;
    try {
      for (const chunk of fixtureArray(testCase.chunks_base64)) {
        frames.push(...splitter.push(decodeBase64(fixtureString(chunk))));
      }
      splitter.finish();
    } catch (error) {
      caught = error;
    }
    const expectedError = testCase.error === undefined ? undefined : fixtureString(testCase.error);
    if (expectedError !== undefined) {
      assert.equal(errorKind(caught), expectedError, fixtureString(testCase.name));
    } else {
      assert.equal(caught, undefined, fixtureString(testCase.name));
    }
    const expectedFrames = (testCase.frames_base64 === undefined ? [] : fixtureArray(testCase.frames_base64))
      .map((item) => decoder.decode(decodeBase64(fixtureString(item))));
    assert.deepEqual(frames.map((frame) => decoder.decode(frame)), expectedFrames, fixtureString(testCase.name));
  }
});
