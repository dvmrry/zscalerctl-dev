import { ERROR_KINDS, fail, WireError } from "./errors.ts";

export type JsonPrimitive = null | boolean | string | WireNumber;
export type JsonObject = { [key: string]: JsonValue };
export type JsonValue = JsonPrimitive | JsonValue[] | JsonObject;

export interface JsonParseOptions {
  readonly maximumBytes?: number;
  readonly maximumDepth?: number;
  readonly requireObject?: boolean;
}

export interface JsonEncodeOptions {
  readonly maximumBytes?: number;
  readonly maximumDepth?: number;
}

const DEFAULT_MAXIMUM_BYTES = 1 * 1024 * 1024;
const DEFAULT_MAXIMUM_DEPTH = 64;
const textDecoder = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true });
const textEncoder = new TextEncoder();

function isDigit(value: number): boolean {
  return value >= 0x30 && value <= 0x39;
}

function isHexDigit(value: number): boolean {
  return (value >= 0x30 && value <= 0x39) ||
    (value >= 0x41 && value <= 0x46) ||
    (value >= 0x61 && value <= 0x66);
}

function validateUtf8(data: Uint8Array): void {
  for (let index = 0; index < data.length; index += 1) {
    const first = data[index];
    if (first <= 0x7f) {
      continue;
    }
    if (first >= 0xc2 && first <= 0xdf) {
      if (index + 1 >= data.length || data[index + 1] < 0x80 || data[index + 1] > 0xbf) {
        fail(ERROR_KINDS.INVALID_UTF8, `invalid UTF-8 at byte ${index}`);
      }
      index += 1;
      continue;
    }
    if (first >= 0xe0 && first <= 0xef) {
      if (index + 2 >= data.length) {
        fail(ERROR_KINDS.INVALID_UTF8, `truncated UTF-8 at byte ${index}`);
      }
      const second = data[index + 1];
      const third = data[index + 2];
      const secondValid = first === 0xe0
        ? second >= 0xa0 && second <= 0xbf
        : first === 0xed
          ? second >= 0x80 && second <= 0x9f
          : second >= 0x80 && second <= 0xbf;
      if (!secondValid || third < 0x80 || third > 0xbf) {
        fail(ERROR_KINDS.INVALID_UTF8, `invalid UTF-8 at byte ${index}`);
      }
      index += 2;
      continue;
    }
    if (first >= 0xf0 && first <= 0xf4) {
      if (index + 3 >= data.length) {
        fail(ERROR_KINDS.INVALID_UTF8, `truncated UTF-8 at byte ${index}`);
      }
      const second = data[index + 1];
      const third = data[index + 2];
      const fourth = data[index + 3];
      const secondValid = first === 0xf0
        ? second >= 0x90 && second <= 0xbf
        : first === 0xf4
          ? second >= 0x80 && second <= 0x8f
          : second >= 0x80 && second <= 0xbf;
      if (!secondValid || third < 0x80 || third > 0xbf || fourth < 0x80 || fourth > 0xbf) {
        fail(ERROR_KINDS.INVALID_UTF8, `invalid UTF-8 at byte ${index}`);
      }
      index += 3;
      continue;
    }
    fail(ERROR_KINDS.INVALID_UTF8, `invalid UTF-8 at byte ${index}`);
  }
}

function validateParseOptions(options: JsonParseOptions): Required<JsonParseOptions> {
  const maximumBytes = options.maximumBytes ?? DEFAULT_MAXIMUM_BYTES;
  const maximumDepth = options.maximumDepth ?? DEFAULT_MAXIMUM_DEPTH;
  if (!Number.isSafeInteger(maximumBytes) || maximumBytes <= 0 || maximumBytes > 0x7fffffff) {
    fail(ERROR_KINDS.INVALID_FRAME, "maximumBytes must be a bounded positive integer");
  }
  if (!Number.isSafeInteger(maximumDepth) || maximumDepth <= 0) {
    fail(ERROR_KINDS.INVALID_FRAME, "maximumDepth must be a positive integer");
  }
  return {
    maximumBytes,
    maximumDepth,
    requireObject: options.requireObject ?? false,
  };
}

function decodeUtf8(data: Uint8Array): string {
  try {
    return textDecoder.decode(data);
  } catch {
    fail(ERROR_KINDS.INVALID_UTF8, "invalid UTF-8 string");
  }
}

function ascii(data: Uint8Array, start: number, end: number): string {
  let value = "";
  for (let index = start; index < end; index += 1) {
    value += String.fromCharCode(data[index]);
  }
  return value;
}

function validNumberLexeme(value: string): boolean {
  let index = 0;
  if (value[index] === "-") {
    index += 1;
  }
  if (value[index] === "0") {
    index += 1;
    if (isDigit(value.charCodeAt(index))) {
      return false;
    }
  } else {
    if (value[index] < "1" || value[index] > "9") {
      return false;
    }
    index += 1;
    while (isDigit(value.charCodeAt(index))) {
      index += 1;
    }
  }
  if (value[index] === ".") {
    index += 1;
    if (!isDigit(value.charCodeAt(index))) {
      return false;
    }
    while (isDigit(value.charCodeAt(index))) {
      index += 1;
    }
  }
  if (value[index] === "e" || value[index] === "E") {
    index += 1;
    if (value[index] === "+" || value[index] === "-") {
      index += 1;
    }
    if (!isDigit(value.charCodeAt(index))) {
      return false;
    }
    while (isDigit(value.charCodeAt(index))) {
      index += 1;
    }
  }
  return index === value.length;
}

export class WireNumber {
  readonly lexeme: string;

  constructor(lexeme: string) {
    if (typeof lexeme !== "string" || !validNumberLexeme(lexeme)) {
      throw new WireError(ERROR_KINDS.INVALID_JSON, "invalid JSON number lexeme");
    }
    this.lexeme = lexeme;
    Object.freeze(this);
  }

  toString(): string {
    return this.lexeme;
  }
}

export function isWireNumber(value: unknown): value is WireNumber {
  return value instanceof WireNumber;
}

class JsonParser {
  readonly data: Uint8Array;
  readonly maximumDepth: number;
  position = 0;

  constructor(data: Uint8Array, maximumDepth: number) {
    this.data = data;
    this.maximumDepth = maximumDepth;
  }

  parse(requireObject: boolean): JsonValue {
    this.skipWhitespace();
    if (this.position >= this.data.length) {
      fail(ERROR_KINDS.INVALID_JSON, "empty JSON value");
    }
    if (requireObject && this.data[this.position] !== 0x7b) {
      fail(ERROR_KINDS.INVALID_JSON, "top-level value must be an object");
    }
    const value = this.parseValue(1);
    this.skipWhitespace();
    if (this.position !== this.data.length) {
      fail(ERROR_KINDS.INVALID_JSON, `trailing JSON at byte ${this.position}`);
    }
    return value;
  }

  private parseValue(depth: number): JsonValue {
    if (this.position >= this.data.length) {
      fail(ERROR_KINDS.INVALID_JSON, "unexpected end of JSON");
    }
    switch (this.data[this.position]) {
      case 0x7b:
        return this.parseObject(depth);
      case 0x5b:
        return this.parseArray(depth);
      case 0x22:
        return this.parseString();
      case 0x74:
        this.parseLiteral("true");
        return true;
      case 0x66:
        this.parseLiteral("false");
        return false;
      case 0x6e:
        this.parseLiteral("null");
        return null;
      default:
        return this.parseNumber();
    }
  }

  private parseObject(depth: number): JsonObject {
    if (depth > this.maximumDepth) {
      fail(ERROR_KINDS.JSON_DEPTH, "object nesting exceeds maximum");
    }
    this.position += 1;
    this.skipWhitespace();
    const object: JsonObject = Object.create(null) as JsonObject;
    if (this.consume(0x7d)) {
      return object;
    }
    const seen = new Set<string>();
    while (true) {
      if (this.position >= this.data.length || this.data[this.position] !== 0x22) {
        fail(ERROR_KINDS.INVALID_JSON, "object key must be a string");
      }
      const key = this.parseString();
      if (seen.has(key)) {
        fail(ERROR_KINDS.DUPLICATE_KEY, "decoded object key is duplicated");
      }
      seen.add(key);
      this.skipWhitespace();
      if (!this.consume(0x3a)) {
        fail(ERROR_KINDS.INVALID_JSON, "object member is missing a colon");
      }
      this.skipWhitespace();
      const value = this.parseValue(depth + 1);
      Object.defineProperty(object, key, {
        configurable: true,
        enumerable: true,
        value,
        writable: true,
      });
      this.skipWhitespace();
      if (this.consume(0x7d)) {
        return object;
      }
      if (!this.consume(0x2c)) {
        fail(ERROR_KINDS.INVALID_JSON, "object member is missing a comma");
      }
      this.skipWhitespace();
    }
  }

  private parseArray(depth: number): JsonValue[] {
    if (depth > this.maximumDepth) {
      fail(ERROR_KINDS.JSON_DEPTH, "array nesting exceeds maximum");
    }
    this.position += 1;
    this.skipWhitespace();
    const array: JsonValue[] = [];
    if (this.consume(0x5d)) {
      return array;
    }
    while (true) {
      array.push(this.parseValue(depth + 1));
      this.skipWhitespace();
      if (this.consume(0x5d)) {
        return array;
      }
      if (!this.consume(0x2c)) {
        fail(ERROR_KINDS.INVALID_JSON, "array value is missing a comma");
      }
      this.skipWhitespace();
    }
  }

  private parseString(): string {
    if (!this.consume(0x22)) {
      fail(ERROR_KINDS.INVALID_JSON, "string must start with a quote");
    }
    const parts: string[] = [];
    let rawStart = this.position;
    while (this.position < this.data.length) {
      const value = this.data[this.position];
      if (value === 0x22) {
        if (rawStart < this.position) {
          parts.push(decodeUtf8(this.data.subarray(rawStart, this.position)));
        }
        this.position += 1;
        return parts.join("");
      }
      if (value === 0x5c) {
        if (rawStart < this.position) {
          parts.push(decodeUtf8(this.data.subarray(rawStart, this.position)));
        }
        this.position += 1;
        if (this.position >= this.data.length) {
          fail(ERROR_KINDS.INVALID_JSON, "unterminated string escape");
        }
        const escape = this.data[this.position];
        this.position += 1;
        switch (escape) {
          case 0x22:
            parts.push('"');
            break;
          case 0x5c:
            parts.push("\\");
            break;
          case 0x2f:
            parts.push("/");
            break;
          case 0x62:
            parts.push("\b");
            break;
          case 0x66:
            parts.push("\f");
            break;
          case 0x6e:
            parts.push("\n");
            break;
          case 0x72:
            parts.push("\r");
            break;
          case 0x74:
            parts.push("\t");
            break;
          case 0x75: {
            const codeUnit = this.parseCodeUnit();
            if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
              if (this.position + 1 >= this.data.length || this.data[this.position] !== 0x5c || this.data[this.position + 1] !== 0x75) {
                fail(ERROR_KINDS.INVALID_JSON, "unpaired high surrogate");
              }
              this.position += 2;
              const low = this.parseCodeUnit();
              if (low < 0xdc00 || low > 0xdfff) {
                fail(ERROR_KINDS.INVALID_JSON, "unpaired high surrogate");
              }
              parts.push(String.fromCodePoint(0x10000 + ((codeUnit - 0xd800) * 0x400) + (low - 0xdc00)));
            } else if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
              fail(ERROR_KINDS.INVALID_JSON, "unpaired low surrogate");
            } else {
              parts.push(String.fromCharCode(codeUnit));
            }
            break;
          }
          default:
            fail(ERROR_KINDS.INVALID_JSON, "unsupported string escape");
        }
        rawStart = this.position;
        continue;
      }
      if (value < 0x20) {
        fail(ERROR_KINDS.INVALID_JSON, "unescaped string control");
      }
      if (value < 0x80) {
        this.position += 1;
      } else {
        this.position += utf8SequenceLength(value);
      }
    }
    fail(ERROR_KINDS.INVALID_JSON, "unterminated string");
  }

  private parseCodeUnit(): number {
    if (this.position + 4 > this.data.length) {
      fail(ERROR_KINDS.INVALID_JSON, "short Unicode escape");
    }
    let value = 0;
    for (let count = 0; count < 4; count += 1) {
      const digit = this.data[this.position];
      if (!isHexDigit(digit)) {
        fail(ERROR_KINDS.INVALID_JSON, "invalid Unicode escape");
      }
      value *= 16;
      if (digit <= 0x39) {
        value += digit - 0x30;
      } else if (digit <= 0x46) {
        value += digit - 0x41 + 10;
      } else {
        value += digit - 0x61 + 10;
      }
      this.position += 1;
    }
    return value;
  }

  private parseLiteral(literal: string): void {
    for (let index = 0; index < literal.length; index += 1) {
      if (this.position >= this.data.length || this.data[this.position] !== literal.charCodeAt(index)) {
        fail(ERROR_KINDS.INVALID_JSON, "invalid JSON literal");
      }
      this.position += 1;
    }
  }

  private parseNumber(): WireNumber {
    const start = this.position;
    if (this.consume(0x2d) && this.position >= this.data.length) {
      fail(ERROR_KINDS.INVALID_JSON, "incomplete number");
    }
    if (this.consume(0x30)) {
      if (isDigit(this.data[this.position])) {
        fail(ERROR_KINDS.INVALID_JSON, "number has a leading zero");
      }
    } else {
      if (!isDigit(this.data[this.position]) || this.data[this.position] === 0x30) {
        fail(ERROR_KINDS.INVALID_JSON, "invalid number integer part");
      }
      while (isDigit(this.data[this.position])) {
        this.position += 1;
      }
    }
    if (this.consume(0x2e)) {
      if (!isDigit(this.data[this.position])) {
        fail(ERROR_KINDS.INVALID_JSON, "invalid number fraction");
      }
      while (isDigit(this.data[this.position])) {
        this.position += 1;
      }
    }
    if (this.data[this.position] === 0x65 || this.data[this.position] === 0x45) {
      this.position += 1;
      if (this.data[this.position] === 0x2b || this.data[this.position] === 0x2d) {
        this.position += 1;
      }
      if (!isDigit(this.data[this.position])) {
        fail(ERROR_KINDS.INVALID_JSON, "invalid number exponent");
      }
      while (isDigit(this.data[this.position])) {
        this.position += 1;
      }
    }
    if (this.position === start) {
      fail(ERROR_KINDS.INVALID_JSON, "invalid JSON value");
    }
    return new WireNumber(ascii(this.data, start, this.position));
  }

  private skipWhitespace(): void {
    while (this.data[this.position] === 0x20 || this.data[this.position] === 0x09) {
      this.position += 1;
    }
  }

  private consume(value: number): boolean {
    if (this.position < this.data.length && this.data[this.position] === value) {
      this.position += 1;
      return true;
    }
    return false;
  }
}

function utf8SequenceLength(first: number): number {
  if (first >= 0xc2 && first <= 0xdf) {
    return 2;
  }
  if (first >= 0xe0 && first <= 0xef) {
    return 3;
  }
  if (first >= 0xf0 && first <= 0xf4) {
    return 4;
  }
  fail(ERROR_KINDS.INVALID_UTF8, "invalid UTF-8 sequence");
}

export function parseJsonValue(data: Uint8Array, options: JsonParseOptions = {}): JsonValue {
  if (!(data instanceof Uint8Array)) {
    fail(ERROR_KINDS.INVALID_FRAME, "JSON input must be a Uint8Array");
  }
  const parsed = validateParseOptions(options);
  if (data.byteLength > parsed.maximumBytes) {
    fail(ERROR_KINDS.FRAME_TOO_LARGE, "JSON input exceeds its byte limit");
  }
  validateUtf8(data);
  if (data.length >= 3 && data[0] === 0xef && data[1] === 0xbb && data[2] === 0xbf) {
    fail(ERROR_KINDS.INVALID_JSON, "UTF-8 BOM is not permitted");
  }
  return new JsonParser(data, parsed.maximumDepth).parse(parsed.requireObject);
}

export function parseJsonObject(data: Uint8Array, options: JsonParseOptions = {}): JsonObject {
  const value = parseJsonValue(data, { ...options, requireObject: true });
  if (Array.isArray(value) || value === null || typeof value !== "object" || value instanceof WireNumber) {
    fail(ERROR_KINDS.INVALID_JSON, "top-level value must be an object");
  }
  return value as JsonObject;
}

export type JsonNode =
  | JsonPrimitive
  | JsonNode[]
  | { [key: string]: JsonNode }
  | OrderedJsonObject;
export type OrderedJsonEntry = readonly [string, JsonNode];

export class OrderedJsonObject {
  readonly entries: readonly OrderedJsonEntry[];

  constructor(entries: readonly OrderedJsonEntry[]) {
    if (!Array.isArray(entries)) {
      fail(ERROR_KINDS.INVALID_FRAME, "ordered JSON entries must be an array");
    }
    const seen = new Set<string>();
    const copied: OrderedJsonEntry[] = [];
    for (const candidate of entries as readonly unknown[]) {
      if (!Array.isArray(candidate) || candidate.length !== 2 || typeof candidate[0] !== "string") {
        fail(ERROR_KINDS.INVALID_FRAME, "ordered JSON entry is invalid");
      }
      const key = candidate[0];
      if (seen.has(key)) {
        fail(ERROR_KINDS.DUPLICATE_KEY, "ordered JSON key is duplicated");
      }
      seen.add(key);
      copied.push(Object.freeze([key, candidate[1]] as const));
    }
    this.entries = copied;
    Object.freeze(this.entries);
    Object.freeze(this);
  }
}

export function orderedJson(entries: readonly OrderedJsonEntry[]): OrderedJsonObject {
  return new OrderedJsonObject(entries);
}

function utf8ByteLength(value: string): number {
  let bytes = 0;
  for (let index = 0; index < value.length; index += 1) {
    const codeUnit = value.charCodeAt(index);
    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      if (index + 1 >= value.length) {
        fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired high surrogate");
      }
      const low = value.charCodeAt(index + 1);
      if (low < 0xdc00 || low > 0xdfff) {
        fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired high surrogate");
      }
      bytes += 4;
      index += 1;
    } else if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
      fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired low surrogate");
    } else if (codeUnit <= 0x7f) {
      bytes += 1;
    } else if (codeUnit <= 0x7ff) {
      bytes += 2;
    } else {
      bytes += 3;
    }
  }
  return bytes;
}

function compareUtf8(left: string, right: string): number {
  const leftBytes = textEncoder.encode(left);
  const rightBytes = textEncoder.encode(right);
  const length = Math.min(leftBytes.length, rightBytes.length);
  for (let index = 0; index < length; index += 1) {
    if (leftBytes[index] !== rightBytes[index]) {
      return leftBytes[index] - rightBytes[index];
    }
  }
  return leftBytes.length - rightBytes.length;
}

class JsonWriter {
  readonly maximumBytes: number;
  readonly maximumDepth: number;
  readonly parts: string[] = [];
  bytes = 0;

  constructor(options: JsonEncodeOptions) {
    this.maximumBytes = options.maximumBytes ?? DEFAULT_MAXIMUM_BYTES;
    this.maximumDepth = options.maximumDepth ?? DEFAULT_MAXIMUM_DEPTH;
    if (!Number.isSafeInteger(this.maximumBytes) || this.maximumBytes <= 0 || this.maximumBytes > 0x7fffffff) {
      fail(ERROR_KINDS.INVALID_FRAME, "maximumBytes must be a bounded positive integer");
    }
    if (!Number.isSafeInteger(this.maximumDepth) || this.maximumDepth <= 0) {
      fail(ERROR_KINDS.INVALID_FRAME, "maximumDepth must be a positive integer");
    }
  }

  append(value: string): void {
    const bytes = utf8ByteLength(value);
    if (this.bytes + bytes > this.maximumBytes) {
      fail(ERROR_KINDS.FRAME_TOO_LARGE, "encoded JSON exceeds its byte limit");
    }
    this.bytes += bytes;
    this.parts.push(value);
  }

  finish(): Uint8Array {
    return textEncoder.encode(this.parts.join(""));
  }
}

function writeString(value: string, writer: JsonWriter): void {
  writer.append('"');
  for (let index = 0; index < value.length; index += 1) {
    const codeUnit = value.charCodeAt(index);
    let codePoint = codeUnit;
    let character = value[index];
    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      if (index + 1 >= value.length) {
        fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired high surrogate");
      }
      const low = value.charCodeAt(index + 1);
      if (low < 0xdc00 || low > 0xdfff) {
        fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired high surrogate");
      }
      codePoint = 0x10000 + ((codeUnit - 0xd800) * 0x400) + (low - 0xdc00);
      character = value.slice(index, index + 2);
      index += 1;
    } else if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
      fail(ERROR_KINDS.INVALID_JSON, "string contains an unpaired low surrogate");
    }
    switch (codePoint) {
      case 0x08:
        writer.append("\\b");
        break;
      case 0x09:
        writer.append("\\t");
        break;
      case 0x0a:
        writer.append("\\n");
        break;
      case 0x0c:
        writer.append("\\f");
        break;
      case 0x0d:
        writer.append("\\r");
        break;
      case 0x22:
        writer.append("\\\"");
        break;
      case 0x5c:
        writer.append("\\\\");
        break;
      default:
        if (codePoint < 0x20) {
          const hex = codePoint.toString(16).padStart(2, "0");
          writer.append(`\\u00${hex}`);
        } else {
          writer.append(character);
        }
    }
  }
  writer.append('"');
}

function enterComposite(value: object, ancestors: Set<object>): void {
  if (ancestors.has(value)) {
    fail(ERROR_KINDS.INVALID_FRAME, "JSON value contains a cycle");
  }
  ancestors.add(value);
}

function writeJson(value: JsonNode, writer: JsonWriter, depth: number, ancestors: Set<object>): void {
  if (value instanceof OrderedJsonObject) {
    if (depth > writer.maximumDepth) {
      fail(ERROR_KINDS.JSON_DEPTH, "encoded object exceeds maximum depth");
    }
    enterComposite(value, ancestors);
    try {
      writer.append("{");
      for (let index = 0; index < value.entries.length; index += 1) {
        const [key, child] = value.entries[index];
        if (index > 0) {
          writer.append(",");
        }
        writeString(key, writer);
        writer.append(":");
        writeJson(child, writer, depth + 1, ancestors);
      }
      writer.append("}");
    } finally {
      ancestors.delete(value);
    }
    return;
  }
  if (value === null) {
    writer.append("null");
    return;
  }
  if (typeof value === "boolean") {
    writer.append(value ? "true" : "false");
    return;
  }
  if (typeof value === "string") {
    writeString(value, writer);
    return;
  }
  if (isWireNumber(value)) {
    writer.append(value.lexeme);
    return;
  }
  if (Array.isArray(value)) {
    if (depth > writer.maximumDepth) {
      fail(ERROR_KINDS.JSON_DEPTH, "encoded array exceeds maximum depth");
    }
    enterComposite(value, ancestors);
    try {
      writer.append("[");
      for (let index = 0; index < value.length; index += 1) {
        if (index > 0) {
          writer.append(",");
        }
        writeJson(value[index], writer, depth + 1, ancestors);
      }
      writer.append("]");
    } finally {
      ancestors.delete(value);
    }
    return;
  }
  if (typeof value === "object") {
    if (depth > writer.maximumDepth) {
      fail(ERROR_KINDS.JSON_DEPTH, "encoded object exceeds maximum depth");
    }
    enterComposite(value, ancestors);
    try {
      const keys = Object.keys(value).sort(compareUtf8);
      writer.append("{");
      for (let index = 0; index < keys.length; index += 1) {
        const key = keys[index];
        if (index > 0) {
          writer.append(",");
        }
        writeString(key, writer);
        writer.append(":");
        writeJson(value[key], writer, depth + 1, ancestors);
      }
      writer.append("}");
    } finally {
      ancestors.delete(value);
    }
    return;
  }
  fail(ERROR_KINDS.INVALID_FRAME, "JSON encoder received an unsupported value");
}

export function encodeJson(value: JsonValue, options: JsonEncodeOptions = {}): Uint8Array {
  const writer = new JsonWriter(options);
  writeJson(value, writer, 1, new Set());
  return writer.finish();
}

export function encodeOrderedJson(entries: readonly OrderedJsonEntry[], options: JsonEncodeOptions = {}): Uint8Array {
  const writer = new JsonWriter(options);
  writeJson(new OrderedJsonObject(entries), writer, 1, new Set());
  return writer.finish();
}

export function structuralNumber(value: number): WireNumber {
  if (!Number.isSafeInteger(value) || value < 0 || value > 9007199254740991) {
    fail(ERROR_KINDS.INVALID_FRAME, "structural number is outside the safe integer range");
  }
  return new WireNumber(String(value));
}
