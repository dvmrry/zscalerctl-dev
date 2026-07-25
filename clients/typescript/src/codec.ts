import {
  AGGREGATE_ITEM_BYTES,
  BOOTSTRAP_FRAME_BYTES,
  BOOTSTRAP_JSON_DEPTH,
  FRAGMENT_CHUNK_BYTES,
  MAX_CONTROL_STRING_BYTES,
  MAX_PATH_BYTES,
  MAX_PRODUCT_SELECTOR_COUNT,
  MAX_READ_FIELD_COUNT,
  MAX_READ_FILTER_COUNT,
  MAX_RESOURCE_SELECTOR_COUNT,
  MAX_SAFE_INTEGER,
  MAX_URL_COUNT,
  PROTOCOL,
  V1_FRAME_BYTES,
  V1_JSON_DEPTH,
  V1_SCHEMA_ID,
  V1_SCHEMA_SHA256,
  V1_VERSION,
} from "./constants.ts";
import { ERROR_KINDS, fail } from "./errors.ts";
import { isUnicodeFormatCodePoint } from "./unicode.ts";
import {
  encodeOrderedJson,
  isWireNumber,
  orderedJson,
  parseJsonObject,
  structuralNumber,
  type JsonNode,
  type JsonObject,
  type JsonValue,
  WireNumber,
} from "./json.ts";
import type {
  AuthStatus,
  BootstrapClientFrame,
  BootstrapError,
  BootstrapProtocolError,
  BootstrapServerFrame,
  Cancel,
  Capability,
  CatalogField,
  CatalogResource,
  Canceled,
  ClientFrame,
  Completed,
  CompletionResult,
  ConfigCredentials,
  ConfigDefaults,
  ConfigProxy,
  ConfigStatus,
  ConfigZIALegacy,
  ConfigZPA,
  DiffCounts,
  DiffFieldChange,
  DiffIdentity,
  DiffInput,
  DiffRecordRef,
  DiffResource,
  DiffSummary,
  DoctorStatus,
  DumpFailure,
  DumpInput,
  Effect,
  EngineCapability,
  EngineManifest,
  Failed,
  FailureKind,
  Filter,
  Hello,
  Initialize,
  ItemBegin,
  ItemChunk,
  ItemEnd,
  ItemFrame,
  ItemKind,
  ItemValue,
  Limits,
  Operation,
  OperationFailure,
  Product,
  Progress,
  ProjectedRecord,
  Ready,
  Redaction,
  Reject,
  ResourceGetInput,
  ResourceListInput,
  ResourceSelector,
  ResourceShowInput,
  SchemaIdentity,
  ServerBuild,
  URLClassification,
  URLLookupInput,
  Warning,
  WireRecord,
} from "./types.ts";

type UnknownObject = Record<string, unknown>;
type Entry = readonly [string, JsonNode];

const PROTOCOL_ERROR_KINDS = new Set(["protocol_violation", "unsupported_protocol", "frame_too_large", "internal"]);
const PRODUCTS = new Set(["zia", "zpa", "ztw", "zcc", "zidentity"]);
const REDACTIONS = new Set(["standard", "share", "paranoid"]);
const ITEM_KINDS = new Set(["catalog_resource", "url_classification", "projected_record", "diff_resource", "diff_added", "diff_removed", "diff_field_change"]);
const CAPABILITIES = new Set(["engine.manifest", "catalog.schema", "status.inspect", "zia.url_lookup", "resources.read", "dump.write", "diff.compare"]);
const OPERATIONS = new Set(["manifest", "list", "get", "show", "doctor", "auth_status", "config_status", "lookup", "dump", "diff"]);
const FAILURE_KINDS = new Set(["usage", "unsupported_capability", "unsupported_operation", "unknown_resource", "not_found", "invalid_resource_id", "live_access_failed", "deadline_exceeded", "invalid_config", "invalid_proxy_config", "unsupported_resource", "response_too_large", "internal"]);
const MISSING_CREDENTIAL_NAMES = new Set(["ZSCALERCTL_CLIENT_ID", "ZSCALERCTL_CLIENT_SECRET", "ZSCALERCTL_VANITY_DOMAIN", "ZSCALERCTL_ZPA_CUSTOMER_ID", "ZSCALERCTL_ZIA_USERNAME", "ZSCALERCTL_ZIA_PASSWORD", "ZSCALERCTL_ZIA_API_KEY", "ZSCALERCTL_ZIA_CLOUD"]);

function hasOwn(value: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function readObject(value: JsonValue | unknown): JsonObject {
  if (typeof value !== "object" || value === null || Array.isArray(value) || isWireNumber(value)) {
    fail(ERROR_KINDS.INVALID_FRAME, "expected a JSON object");
  }
  return value as JsonObject;
}

function readExact(value: JsonValue | unknown, required: readonly string[], optional: readonly string[] = []): JsonObject {
  const object = readObject(value);
  const allowed = new Set([...required, ...optional]);
  for (const key of Object.keys(object)) {
    if (!allowed.has(key)) {
      fail(ERROR_KINDS.INVALID_FRAME, `unknown member ${key}`);
    }
  }
  for (const key of required) {
    if (!hasOwn(object, key)) {
      fail(ERROR_KINDS.INVALID_FRAME, `missing required member ${key}`);
    }
  }
  return object;
}

function readString(value: unknown): string {
  if (typeof value !== "string") {
    fail(ERROR_KINDS.INVALID_FRAME, "expected a string");
  }
  return value;
}

function readBoolean(value: unknown): boolean {
  if (typeof value !== "boolean") {
    fail(ERROR_KINDS.INVALID_FRAME, "expected a boolean");
  }
  return value;
}

function readArray(value: unknown): JsonValue[] {
  if (!Array.isArray(value)) {
    fail(ERROR_KINDS.INVALID_FRAME, "expected an array");
  }
  return value;
}

function typedObject(value: unknown): UnknownObject {
  if (typeof value !== "object" || value === null || Array.isArray(value) || isWireNumber(value)) {
    fail(ERROR_KINDS.INVALID_FRAME, "frame must be an object");
  }
  return value as UnknownObject;
}

function typedExact(value: unknown, required: readonly string[], optional: readonly string[] = []): UnknownObject {
  const object = typedObject(value);
  const allowed = new Set([...required, ...optional]);
  for (const key of Object.keys(object)) {
    if (!allowed.has(key)) {
      fail(ERROR_KINDS.INVALID_FRAME, `unknown member ${key}`);
    }
  }
  for (const key of required) {
    if (!hasOwn(object, key)) {
      fail(ERROR_KINDS.INVALID_FRAME, `missing required member ${key}`);
    }
  }
  return object;
}

function typedString(value: unknown): string {
  return readString(value);
}

function typedBoolean(value: unknown): boolean {
  return readBoolean(value);
}

function typedArray(value: unknown): readonly unknown[] {
  if (!Array.isArray(value)) {
    fail(ERROR_KINDS.INVALID_FRAME, "expected an array");
  }
  return value;
}

function typedInteger(value: unknown, positive = false): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0 || value > MAX_SAFE_INTEGER || (positive && value === 0)) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid structural integer");
  }
  return value;
}

function utf8Length(value: string): number {
  let bytes = 0;
  for (let index = 0; index < value.length; index += 1) {
    const codeUnit = value.charCodeAt(index);
    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      if (index + 1 >= value.length) {
        fail(ERROR_KINDS.INVALID_FRAME, "string contains an unpaired surrogate");
      }
      const low = value.charCodeAt(index + 1);
      if (low < 0xdc00 || low > 0xdfff) {
        fail(ERROR_KINDS.INVALID_FRAME, "string contains an unpaired surrogate");
      }
      bytes += 4;
      index += 1;
    } else if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
      fail(ERROR_KINDS.INVALID_FRAME, "string contains an unpaired surrogate");
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

function structuralString(value: unknown, minimumRunes: number, maximumRunes: number, maximumBytes: number): string {
  const text = readString(value);
  const bytes = utf8Length(text);
  let runes = 0;
  for (let index = 0; index < text.length; index += 1) {
    const codeUnit = text.charCodeAt(index);
    let codePoint = codeUnit;
    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      codePoint = text.codePointAt(index) as number;
      index += 1;
    }
    runes += 1;
    if (codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f) || isUnicodeFormatCodePoint(codePoint)) {
      fail(ERROR_KINDS.INVALID_FRAME, "string contains a forbidden control or format rune");
    }
  }
  if (runes < minimumRunes || (maximumRunes > 0 && runes > maximumRunes) || (maximumBytes > 0 && bytes > maximumBytes)) {
    fail(ERROR_KINDS.INVALID_FRAME, "string length is outside its bound");
  }
  return text;
}

function controlString(value: unknown): string {
  return structuralString(value, 0, MAX_CONTROL_STRING_BYTES, MAX_CONTROL_STRING_BYTES);
}

function resourceName(value: unknown): string {
  return structuralString(value, 1, 128, MAX_CONTROL_STRING_BYTES);
}

function fieldName(value: unknown): string {
  return structuralString(value, 1, 256, MAX_CONTROL_STRING_BYTES);
}

function pathInput(value: unknown): string {
  return structuralString(value, 1, MAX_PATH_BYTES, MAX_PATH_BYTES);
}

function validateDynamicKey(value: string): string {
  return structuralString(value, 0, 0, 0);
}

function versionToken(value: unknown): string {
  const text = readString(value);
  if (text.length < 1 || text.length > 32) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid version token length");
  }
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index);
    const alphanumeric = (code >= 0x41 && code <= 0x5a) || (code >= 0x61 && code <= 0x7a) || (code >= 0x30 && code <= 0x39);
    if (!alphanumeric && (index === 0 || ![0x2e, 0x5f, 0x2d].includes(code))) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid version token");
    }
  }
  return text;
}

function product(value: unknown): Product {
  const text = readString(value);
  if (!PRODUCTS.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown product");
  }
  return text as Product;
}

function redaction(value: unknown): Redaction {
  const text = readString(value);
  if (!REDACTIONS.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown redaction mode");
  }
  return text as Redaction;
}

function capability(value: unknown): Capability {
  const text = readString(value);
  if (!CAPABILITIES.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown capability");
  }
  return text as Capability;
}

function operation(value: unknown): Operation {
  const text = readString(value);
  if (!OPERATIONS.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown operation");
  }
  return text as Operation;
}

function safeInteger(value: unknown, positive = false): number {
  if (isWireNumber(value)) {
    const lexeme = value.lexeme;
    let mantissa = lexeme;
    let exponent = 0;
    const exponentIndex = Math.max(mantissa.indexOf("e"), mantissa.indexOf("E"));
    if (exponentIndex >= 0) {
      exponent = boundedExponent(mantissa.slice(exponentIndex + 1));
      mantissa = mantissa.slice(0, exponentIndex);
    }
    const negative = mantissa.startsWith("-");
    if (negative) {
      mantissa = mantissa.slice(1);
    }
    const dot = mantissa.indexOf(".");
    const fractionDigits = dot >= 0 ? mantissa.length - dot - 1 : 0;
    if (dot >= 0) {
      mantissa = mantissa.slice(0, dot) + mantissa.slice(dot + 1);
    }
    let nonzero = false;
    for (let index = 0; index < mantissa.length; index += 1) {
      if (mantissa[index] !== "0") {
        nonzero = true;
        break;
      }
    }
    if (!nonzero) {
      if (positive) {
        fail(ERROR_KINDS.INVALID_FRAME, "structural integer must be positive");
      }
      return 0;
    }
    if (negative) {
      fail(ERROR_KINDS.INVALID_FRAME, "structural integer must not be negative");
    }
    const shift = exponent - fractionDigits;
    if (shift < 0) {
      const trim = -shift;
      if (trim > mantissa.length) {
        fail(ERROR_KINDS.INVALID_FRAME, "structural number is not integral");
      }
      for (let index = mantissa.length - trim; index < mantissa.length; index += 1) {
        if (mantissa[index] !== "0") {
          fail(ERROR_KINDS.INVALID_FRAME, "structural number is not integral");
        }
      }
      mantissa = mantissa.slice(0, mantissa.length - trim);
    } else if (shift > 0) {
      let significant = 0;
      while (significant < mantissa.length && mantissa[significant] === "0") {
        significant += 1;
      }
      if (shift > 16 || mantissa.length - significant + shift > 16) {
        fail(ERROR_KINDS.INVALID_FRAME, "structural integer exceeds safe range");
      }
      mantissa += "0".repeat(shift);
    }
    let first = 0;
    while (first < mantissa.length && mantissa[first] === "0") {
      first += 1;
    }
    mantissa = mantissa.slice(first);
    if (mantissa.length === 0) {
      return 0;
    }
    if (mantissa.length > 16) {
      fail(ERROR_KINDS.INVALID_FRAME, "structural integer exceeds safe range");
    }
    let result = 0;
    for (let index = 0; index < mantissa.length; index += 1) {
      result = result * 10 + (mantissa.charCodeAt(index) - 0x30);
    }
    if (result > MAX_SAFE_INTEGER || (positive && result === 0)) {
      fail(ERROR_KINDS.INVALID_FRAME, "structural integer is outside its range");
    }
    return result;
  }
  fail(ERROR_KINDS.INVALID_FRAME, "structural integer must be a JSON number");
}

function boundedExponent(value: string): number {
  let sign = 1;
  let index = 0;
  if (value[index] === "+") {
    index += 1;
  } else if (value[index] === "-") {
    sign = -1;
    index += 1;
  }
  let result = 0;
  for (; index < value.length; index += 1) {
    if (result > 1000) {
      return sign * 1001;
    }
    result = result * 10 + value.charCodeAt(index) - 0x30;
  }
  return sign * result;
}

function encodedInteger(value: unknown, positive = false): WireNumber {
  return structuralNumber(typedInteger(value, positive));
}

function protocolErrorKind(value: unknown): BootstrapError["kind"] {
  const text = readString(value);
  if (!PROTOCOL_ERROR_KINDS.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown protocol error kind");
  }
  return text as BootstrapError["kind"];
}

function readStringList(value: unknown, maximum: number, validator: (item: unknown) => string, unique: boolean): string[] {
  const items = readArray(value);
  if (items.length > maximum) {
    fail(ERROR_KINDS.INVALID_FRAME, "array exceeds its limit");
  }
  const output: string[] = [];
  const seen = new Set<string>();
  for (const item of items) {
    const text = validator(item);
    if (unique && seen.has(text)) {
      fail(ERROR_KINDS.INVALID_FRAME, "array contains a duplicate");
    }
    seen.add(text);
    output.push(text);
  }
  return output;
}

function readProtocolError(value: unknown): BootstrapError {
  const object = readExact(value, ["kind"]);
  return { kind: protocolErrorKind(object.kind) };
}

function readBootstrapLimits(value: unknown): { frame_bytes: number; json_depth: number } {
  const object = readExact(value, ["frame_bytes", "json_depth"]);
  const frameBytes = safeInteger(object.frame_bytes);
  const jsonDepth = safeInteger(object.json_depth);
  if (frameBytes !== BOOTSTRAP_FRAME_BYTES || jsonDepth !== BOOTSTRAP_JSON_DEPTH) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid bootstrap limits");
  }
  return { frame_bytes: frameBytes, json_depth: jsonDepth };
}

function entry(key: string, value: JsonNode): Entry {
  return [key, value];
}

function bootstrapInitializeNode(frame: Initialize): JsonNode {
  const object = typedExact(frame, ["type", "protocol", "version"]);
  if (typedString(object.type) !== "initialize" || typedString(object.protocol) !== PROTOCOL) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid initialize discriminants");
  }
  const version = versionToken(object.version);
  return orderedJson([entry("type", "initialize"), entry("protocol", PROTOCOL), entry("version", version)]);
}

function bootstrapRejectNode(frame: Reject): JsonNode {
  const object = typedExact(frame, ["type", "protocol", "reason"]);
  if (typedString(object.type) !== "reject" || typedString(object.protocol) !== PROTOCOL || typedString(object.reason) !== "unsupported_protocol") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid bootstrap rejection");
  }
  return orderedJson([entry("type", "reject"), entry("protocol", PROTOCOL), entry("reason", "unsupported_protocol")]);
}

function bootstrapHelloNode(frame: Hello): JsonNode {
  const object = typedExact(frame, ["type", "protocol", "versions", "bootstrap"]);
  if (typedString(object.type) !== "hello" || typedString(object.protocol) !== PROTOCOL) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid hello discriminants");
  }
  const versions = readStringList(object.versions, 16, versionToken, true);
  if (versions.length < 1) {
    fail(ERROR_KINDS.INVALID_FRAME, "hello must advertise a version");
  }
  const limits = readBootstrapLimitsTyped(object.bootstrap);
  return orderedJson([
    entry("type", "hello"), entry("protocol", PROTOCOL),
    entry("versions", versions),
    entry("bootstrap", orderedJson([entry("frame_bytes", structuralNumber(limits.frame_bytes)), entry("json_depth", structuralNumber(limits.json_depth))])),
  ]);
}

function readBootstrapLimitsTyped(value: unknown): { frame_bytes: number; json_depth: number } {
  const object = typedExact(value, ["frame_bytes", "json_depth"]);
  const frameBytes = typedInteger(object.frame_bytes);
  const jsonDepth = typedInteger(object.json_depth);
  if (frameBytes !== BOOTSTRAP_FRAME_BYTES || jsonDepth !== BOOTSTRAP_JSON_DEPTH) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid bootstrap limits");
  }
  return { frame_bytes: frameBytes, json_depth: jsonDepth };
}

function bootstrapProtocolErrorNode(frame: BootstrapProtocolError): JsonNode {
  const object = typedExact(frame, ["type", "fatal", "error"]);
  if (typedString(object.type) !== "protocol_error" || typedBoolean(object.fatal) !== true) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid protocol error envelope");
  }
  const error = typedExact(object.error, ["kind"]);
  const kind = protocolErrorKind(error.kind);
  return orderedJson([entry("type", "protocol_error"), entry("fatal", true), entry("error", orderedJson([entry("kind", kind)]))]);
}

export function decodeBootstrapClientFrame(data: Uint8Array): BootstrapClientFrame {
  const object = parseJsonObject(data, { maximumBytes: BOOTSTRAP_FRAME_BYTES, maximumDepth: BOOTSTRAP_JSON_DEPTH });
  const type = readString(object.type);
  if (type === "hello" || type === "protocol_error") {
    fail(ERROR_KINDS.WRONG_DIRECTION, "server bootstrap frame sent to client");
  }
  if (type === "initialize") {
    const frame = readExact(object, ["type", "protocol", "version"]);
    if (readString(frame.protocol) !== PROTOCOL) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid initialize protocol");
    }
    return { type: "initialize", protocol: PROTOCOL, version: versionToken(frame.version) };
  }
  if (type === "reject") {
    const frame = readExact(object, ["type", "protocol", "reason"]);
    if (readString(frame.protocol) !== PROTOCOL || readString(frame.reason) !== "unsupported_protocol") {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid bootstrap rejection");
    }
    return { type: "reject", protocol: PROTOCOL, reason: "unsupported_protocol" };
  }
  fail(ERROR_KINDS.INVALID_FRAME, "unknown client bootstrap frame type");
}

export function decodeBootstrapServerFrame(data: Uint8Array): BootstrapServerFrame {
  const object = parseJsonObject(data, { maximumBytes: BOOTSTRAP_FRAME_BYTES, maximumDepth: BOOTSTRAP_JSON_DEPTH });
  const type = readString(object.type);
  if (type === "initialize" || type === "reject") {
    fail(ERROR_KINDS.WRONG_DIRECTION, "client bootstrap frame sent to server");
  }
  if (type === "hello") {
    const frame = readExact(object, ["type", "protocol", "versions", "bootstrap"]);
    if (readString(frame.protocol) !== PROTOCOL) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid hello protocol");
    }
    const versions = readStringList(frame.versions, 16, versionToken, true);
    if (versions.length < 1) {
      fail(ERROR_KINDS.INVALID_FRAME, "hello must advertise a version");
    }
    return { type: "hello", protocol: PROTOCOL, versions, bootstrap: readBootstrapLimits(frame.bootstrap) };
  }
  if (type === "protocol_error") {
    const frame = readExact(object, ["type", "fatal", "error"]);
    if (readBoolean(frame.fatal) !== true) {
      fail(ERROR_KINDS.INVALID_FRAME, "protocol error must be fatal");
    }
    return { type: "protocol_error", fatal: true, error: readProtocolError(frame.error) };
  }
  fail(ERROR_KINDS.INVALID_FRAME, "unknown server bootstrap frame type");
}

export function encodeBootstrapClientFrame(frame: BootstrapClientFrame): Uint8Array {
  const node = frame.type === "initialize" ? bootstrapInitializeNode(frame) : frame.type === "reject" ? bootstrapRejectNode(frame) : fail(ERROR_KINDS.INVALID_FRAME, "unknown client bootstrap frame type");
  return encodeOrderedJson((node as { entries: readonly Entry[] }).entries, { maximumBytes: BOOTSTRAP_FRAME_BYTES, maximumDepth: BOOTSTRAP_JSON_DEPTH });
}

export function encodeBootstrapServerFrame(frame: BootstrapServerFrame): Uint8Array {
  const node = frame.type === "hello" ? bootstrapHelloNode(frame) : frame.type === "protocol_error" ? bootstrapProtocolErrorNode(frame) : fail(ERROR_KINDS.INVALID_FRAME, "unknown server bootstrap frame type");
  return encodeOrderedJson((node as { entries: readonly Entry[] }).entries, { maximumBytes: BOOTSTRAP_FRAME_BYTES, maximumDepth: BOOTSTRAP_JSON_DEPTH });
}

export const decodeBootstrapClient = decodeBootstrapClientFrame;
export const decodeBootstrapServer = decodeBootstrapServerFrame;
export const encodeBootstrapClient = encodeBootstrapClientFrame;
export const encodeBootstrapServer = encodeBootstrapServerFrame;

function validateCapabilityOperation(capabilityValue: unknown, operationValue: unknown): { capability: Capability; operation: Operation } {
  const capabilityName = capability(capabilityValue);
  const operationName = operation(operationValue);
  const valid = capabilityName === "engine.manifest" && operationName === "manifest" ||
    capabilityName === "catalog.schema" && operationName === "list" ||
    capabilityName === "status.inspect" && ["doctor", "auth_status", "config_status"].includes(operationName) ||
    capabilityName === "zia.url_lookup" && operationName === "lookup" ||
    capabilityName === "resources.read" && ["list", "get", "show"].includes(operationName) ||
    capabilityName === "dump.write" && operationName === "dump" ||
    capabilityName === "diff.compare" && operationName === "diff";
  if (!valid) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid capability/operation pair");
  }
  return { capability: capabilityName, operation: operationName };
}

function requestHeader(object: JsonObject): { id: number; capability: Capability; operation: Operation } {
  if (readString(object.type) !== "request") {
    fail(ERROR_KINDS.INVALID_FRAME, "request type must be request");
  }
  const id = safeInteger(object.id, true);
  return { id, ...validateCapabilityOperation(object.capability, object.operation) };
}

function readFieldSelection(value: unknown): string[] {
  return readStringList(value, MAX_READ_FIELD_COUNT, (item) => fieldName(item), true);
}

function typedFieldSelection(value: unknown): string[] {
  const items = typedArray(value);
  if (items.length > MAX_READ_FIELD_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "fields exceed their limit");
  }
  const output: string[] = [];
  const seen = new Set<string>();
  for (const item of items) {
    const field = fieldName(item);
    if (seen.has(field)) {
      fail(ERROR_KINDS.INVALID_FRAME, "fields contain a duplicate");
    }
    seen.add(field);
    output.push(field);
  }
  return output;
}

function readResourceSelector(value: unknown): ResourceSelector {
  const object = readExact(value, ["product", "resource"]);
  return { product: product(object.product), resource: resourceName(object.resource) };
}

function typedResourceSelector(value: unknown): JsonNode {
  const object = typedExact(value, ["product", "resource"]);
  return orderedJson([entry("product", product(object.product)), entry("resource", resourceName(object.resource))]);
}

function readSelections(productsValue: unknown, resourcesValue: unknown): { products: Product[]; resources: ResourceSelector[] } {
  const productValues = readArray(productsValue);
  if (productValues.length > MAX_PRODUCT_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "products exceed their limit");
  }
  const products: Product[] = [];
  const seenProducts = new Set<Product>();
  for (const value of productValues) {
    const item = product(value);
    if (seenProducts.has(item)) {
      fail(ERROR_KINDS.INVALID_FRAME, "products contain a duplicate");
    }
    seenProducts.add(item);
    products.push(item);
  }
  const resourceValues = readArray(resourcesValue);
  if (resourceValues.length > MAX_RESOURCE_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "resources exceed their limit");
  }
  const resources: ResourceSelector[] = [];
  const seenResources = new Set<string>();
  for (const value of resourceValues) {
    const item = readResourceSelector(value);
    const key = `${item.product}\u0000${item.resource}`;
    if (seenResources.has(key)) {
      fail(ERROR_KINDS.INVALID_FRAME, "resources contain a duplicate");
    }
    seenResources.add(key);
    resources.push(item);
  }
  return { products, resources };
}

function typedSelections(productsValue: unknown, resourcesValue: unknown): { products: Product[]; resources: JsonNode[] } {
  const productValues = typedArray(productsValue);
  if (productValues.length > MAX_PRODUCT_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "products exceed their limit");
  }
  const products: Product[] = [];
  const seenProducts = new Set<Product>();
  for (const value of productValues) {
    const item = product(value);
    if (seenProducts.has(item)) {
      fail(ERROR_KINDS.INVALID_FRAME, "products contain a duplicate");
    }
    seenProducts.add(item);
    products.push(item);
  }
  const resourceValues = typedArray(resourcesValue);
  if (resourceValues.length > MAX_RESOURCE_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "resources exceed their limit");
  }
  const resources: JsonNode[] = [];
  const seenResources = new Set<string>();
  for (const value of resourceValues) {
    const object = typedObject(value);
    const item = { product: product(object.product), resource: resourceName(object.resource) };
    const key = `${item.product}\u0000${item.resource}`;
    if (seenResources.has(key)) {
      fail(ERROR_KINDS.INVALID_FRAME, "resources contain a duplicate");
    }
    seenResources.add(key);
    resources.push(typedResourceSelector(value));
  }
  return { products, resources };
}

function readFilter(value: unknown): Filter {
  const object = readExact(value, ["field", "operator", "value"]);
  const filterOperator = readString(object.operator);
  if (filterOperator !== "exact" && filterOperator !== "contains") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid filter operator");
  }
  return { field: fieldName(object.field), operator: filterOperator, value: controlString(object.value) };
}

function typedFilter(value: unknown): JsonNode {
  const object = typedExact(value, ["field", "operator", "value"]);
  const filterOperator = typedString(object.operator);
  if (filterOperator !== "exact" && filterOperator !== "contains") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid filter operator");
  }
  return orderedJson([entry("field", fieldName(object.field)), entry("operator", filterOperator), entry("value", controlString(object.value))]);
}

function readURLInput(value: unknown): URLLookupInput {
  const object = readExact(value, ["urls"]);
  const urls = readStringList(object.urls, MAX_URL_COUNT, (item) => structuralString(item, 1, MAX_CONTROL_STRING_BYTES, MAX_CONTROL_STRING_BYTES), false);
  if (urls.length < 1) {
    fail(ERROR_KINDS.INVALID_FRAME, "URL lookup requires at least one URL");
  }
  return { urls };
}

function typedURLInput(value: unknown): JsonNode {
  const object = typedExact(value, ["urls"]);
  const urls = typedArray(object.urls);
  if (urls.length < 1 || urls.length > MAX_URL_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "URL lookup URL count is outside its limit");
  }
  const encoded = urls.map((item) => structuralString(item, 1, MAX_CONTROL_STRING_BYTES, MAX_CONTROL_STRING_BYTES));
  return orderedJson([entry("urls", encoded)]);
}

function readResourceListInput(value: unknown): ResourceListInput {
  const object = readExact(value, ["product", "resource", "fields", "filters", "search"]);
  const filters = readArray(object.filters);
  if (filters.length > MAX_READ_FILTER_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "filters exceed their limit");
  }
  return {
    product: product(object.product),
    resource: resourceName(object.resource),
    fields: readFieldSelection(object.fields),
    filters: filters.map((item) => readFilter(item)),
    search: controlString(object.search),
  };
}

function typedResourceListInput(value: unknown): JsonNode {
  const object = typedExact(value, ["product", "resource", "fields", "filters", "search"]);
  const filters = typedArray(object.filters);
  if (filters.length > MAX_READ_FILTER_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "filters exceed their limit");
  }
  return orderedJson([
    entry("product", product(object.product)), entry("resource", resourceName(object.resource)),
    entry("fields", typedFieldSelection(object.fields)), entry("filters", filters.map((item) => typedFilter(item))),
    entry("search", controlString(object.search)),
  ]);
}

function readResourceGetInput(value: unknown): ResourceGetInput {
  const object = readExact(value, ["product", "resource", "record_id", "fields"]);
  return { product: product(object.product), resource: resourceName(object.resource), record_id: structuralString(object.record_id, 1, MAX_CONTROL_STRING_BYTES, MAX_CONTROL_STRING_BYTES), fields: readFieldSelection(object.fields) };
}

function typedResourceGetInput(value: unknown): JsonNode {
  const object = typedExact(value, ["product", "resource", "record_id", "fields"]);
  return orderedJson([
    entry("product", product(object.product)), entry("resource", resourceName(object.resource)),
    entry("record_id", structuralString(object.record_id, 1, MAX_CONTROL_STRING_BYTES, MAX_CONTROL_STRING_BYTES)),
    entry("fields", typedFieldSelection(object.fields)),
  ]);
}

function readResourceShowInput(value: unknown): ResourceShowInput {
  const object = readExact(value, ["product", "resource", "fields"]);
  return { product: product(object.product), resource: resourceName(object.resource), fields: readFieldSelection(object.fields) };
}

function typedResourceShowInput(value: unknown): JsonNode {
  const object = typedExact(value, ["product", "resource", "fields"]);
  return orderedJson([entry("product", product(object.product)), entry("resource", resourceName(object.resource)), entry("fields", typedFieldSelection(object.fields))]);
}

function readDumpInput(value: unknown): DumpInput {
  const object = readExact(value, ["output_dir", "products", "resources", "continue_on_error", "force"]);
  const selections = readSelections(object.products, object.resources);
  return { output_dir: pathInput(object.output_dir), products: selections.products, resources: selections.resources, continue_on_error: readBoolean(object.continue_on_error), force: readBoolean(object.force) };
}

function typedDumpInput(value: unknown): JsonNode {
  const object = typedExact(value, ["output_dir", "products", "resources", "continue_on_error", "force"]);
  const selections = typedSelections(object.products, object.resources);
  return orderedJson([
    entry("output_dir", pathInput(object.output_dir)), entry("products", selections.products), entry("resources", selections.resources),
    entry("continue_on_error", typedBoolean(object.continue_on_error)), entry("force", typedBoolean(object.force)),
  ]);
}

function readDiffInput(value: unknown): DiffInput {
  const object = readExact(value, ["old_dir", "new_dir", "products", "resources", "ignore_operational", "allow_partial"]);
  const selections = readSelections(object.products, object.resources);
  return { old_dir: pathInput(object.old_dir), new_dir: pathInput(object.new_dir), products: selections.products, resources: selections.resources, ignore_operational: readBoolean(object.ignore_operational), allow_partial: readBoolean(object.allow_partial) };
}

function typedDiffInput(value: unknown): JsonNode {
  const object = typedExact(value, ["old_dir", "new_dir", "products", "resources", "ignore_operational", "allow_partial"]);
  const selections = typedSelections(object.products, object.resources);
  return orderedJson([
    entry("old_dir", pathInput(object.old_dir)), entry("new_dir", pathInput(object.new_dir)), entry("products", selections.products), entry("resources", selections.resources),
    entry("ignore_operational", typedBoolean(object.ignore_operational)), entry("allow_partial", typedBoolean(object.allow_partial)),
  ]);
}

function clientRequestNode(frame: Exclude<ClientFrame, Cancel>): JsonNode {
  const object = typedObject(frame);
  const type = typedString(object.type);
  if (type !== "request") {
    fail(ERROR_KINDS.INVALID_FRAME, "request type must be request");
  }
  const id = typedInteger(object.id, true);
  const capabilityValue = typedString(object.capability);
  const operationValue = typedString(object.operation);
  const common = [entry("type", "request"), entry("id", structuralNumber(id)), entry("capability", capabilityValue), entry("operation", operationValue)] as Entry[];
  if (capabilityValue === "engine.manifest" && operationValue === "manifest") {
    typedExact(frame, ["type", "id", "capability", "operation"]);
    return orderedJson(common);
  }
  if (capabilityValue === "catalog.schema" && operationValue === "list") {
    typedExact(frame, ["type", "id", "capability", "operation"]);
    return orderedJson(common);
  }
  if (capabilityValue === "status.inspect" && ["doctor", "auth_status", "config_status"].includes(operationValue)) {
    typedExact(frame, ["type", "id", "capability", "operation"]);
    return orderedJson(common);
  }
  if (capabilityValue === "zia.url_lookup" && operationValue === "lookup") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedURLInput(object.input)));
    return orderedJson(common);
  }
  if (capabilityValue === "resources.read" && operationValue === "list") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedResourceListInput(object.input)));
    return orderedJson(common);
  }
  if (capabilityValue === "resources.read" && operationValue === "get") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedResourceGetInput(object.input)));
    return orderedJson(common);
  }
  if (capabilityValue === "resources.read" && operationValue === "show") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedResourceShowInput(object.input)));
    return orderedJson(common);
  }
  if (capabilityValue === "dump.write" && operationValue === "dump") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedDumpInput(object.input)));
    return orderedJson(common);
  }
  if (capabilityValue === "diff.compare" && operationValue === "diff") {
    typedExact(frame, ["type", "id", "capability", "operation", "input"]);
    common.push(entry("input", typedDiffInput(object.input)));
    return orderedJson(common);
  }
  fail(ERROR_KINDS.INVALID_FRAME, "invalid capability/operation pair");
}

function cancelNode(frame: Cancel): JsonNode {
  const object = typedExact(frame, ["type", "id"]);
  if (typedString(object.type) !== "cancel") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid cancel type");
  }
  return orderedJson([entry("type", "cancel"), entry("id", encodedInteger(object.id, true))]);
}

export function decodeClientFrame(data: Uint8Array): ClientFrame {
  const object = parseJsonObject(data, { maximumBytes: V1_FRAME_BYTES, maximumDepth: V1_JSON_DEPTH });
  const type = readString(object.type);
  if (type === "cancel") {
    const frame = readExact(object, ["type", "id"]);
    return { type: "cancel", id: safeInteger(frame.id, true) };
  }
  if (type !== "request") {
    if (["ready", "request_rejected", "started", "item", "item_begin", "item_chunk", "item_end", "progress", "warning", "completed", "failed", "canceled", "protocol_error"].includes(type)) {
      fail(ERROR_KINDS.WRONG_DIRECTION, "server frame sent to client");
    }
    fail(ERROR_KINDS.INVALID_FRAME, "unknown client frame type");
  }
  const header = requestHeader(object);
  const common = { type: "request" as const, id: header.id };
  if (header.capability === "engine.manifest") {
    readExact(object, ["type", "id", "capability", "operation"]);
    return { ...common, capability: "engine.manifest", operation: "manifest" };
  }
  if (header.capability === "catalog.schema") {
    readExact(object, ["type", "id", "capability", "operation"]);
    return { ...common, capability: "catalog.schema", operation: "list" };
  }
  if (header.capability === "status.inspect") {
    readExact(object, ["type", "id", "capability", "operation"]);
    if (header.operation === "doctor") return { ...common, capability: "status.inspect", operation: "doctor" };
    if (header.operation === "auth_status") return { ...common, capability: "status.inspect", operation: "auth_status" };
    return { ...common, capability: "status.inspect", operation: "config_status" };
  }
  if (header.capability === "zia.url_lookup") {
    const frame = readExact(object, ["type", "id", "capability", "operation", "input"]);
    return { ...common, capability: "zia.url_lookup", operation: "lookup", input: readURLInput(frame.input) };
  }
  if (header.capability === "resources.read") {
    const frame = readExact(object, ["type", "id", "capability", "operation", "input"]);
    if (header.operation === "list") return { ...common, capability: "resources.read", operation: "list", input: readResourceListInput(frame.input) };
    if (header.operation === "get") return { ...common, capability: "resources.read", operation: "get", input: readResourceGetInput(frame.input) };
    return { ...common, capability: "resources.read", operation: "show", input: readResourceShowInput(frame.input) };
  }
  if (header.capability === "dump.write") {
    const frame = readExact(object, ["type", "id", "capability", "operation", "input"]);
    return { ...common, capability: "dump.write", operation: "dump", input: readDumpInput(frame.input) };
  }
  const frame = readExact(object, ["type", "id", "capability", "operation", "input"]);
  return { ...common, capability: "diff.compare", operation: "diff", input: readDiffInput(frame.input) };
}

export function encodeClientFrame(frame: ClientFrame): Uint8Array {
  const node = frame.type === "cancel" ? cancelNode(frame) : clientRequestNode(frame);
  return encodeOrderedJson((node as { entries: readonly Entry[] }).entries, { maximumBytes: V1_FRAME_BYTES, maximumDepth: V1_JSON_DEPTH });
}

export const decodeV1ClientFrame = decodeClientFrame;
export const encodeV1ClientFrame = encodeClientFrame;

function readHex(value: unknown, length: number): string {
  const text = readString(value);
  if (text.length !== length) {
    fail(ERROR_KINDS.INVALID_FRAME, "hex string has the wrong length");
  }
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index);
    const digit = code >= 0x30 && code <= 0x39;
    const lower = code >= 0x61 && code <= 0x66;
    if (!digit && !lower) {
      fail(ERROR_KINDS.INVALID_FRAME, "hex string must be lowercase hexadecimal");
    }
  }
  return text;
}

function readItemKind(value: unknown): ItemKind {
  const text = readString(value);
  if (!ITEM_KINDS.has(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown item kind");
  }
  return text as ItemKind;
}

function readFieldClassification(value: unknown): string {
  const text = readString(value);
  if (!["public_project_data", "operational_metadata", "tenant_configuration", "sensitive_identifier", "free_text", "secret"].includes(text)) {
    fail(ERROR_KINDS.INVALID_FRAME, "unknown field classification");
  }
  return text;
}

function readCatalogField(value: unknown): CatalogField {
  const object = readExact(value, ["name", "classification", "allowed_modes", "fields"], ["json_name"]);
  const allowedModes = readStringList(object.allowed_modes, 3, redaction, true);
  const nested = readArray(object.fields);
  if (nested.length > MAX_RESOURCE_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "catalog fields exceed their limit");
  }
  const field: { name: string; json_name?: string; classification: string; allowed_modes: Redaction[]; fields: CatalogField[] } = {
    name: fieldName(object.name),
    classification: readFieldClassification(object.classification),
    allowed_modes: allowedModes as Redaction[],
    fields: nested.map((item) => readCatalogField(item)),
  };
  if (hasOwn(object, "json_name")) {
    field.json_name = fieldName(object.json_name);
  }
  return field as CatalogField;
}

function readCatalogResource(value: unknown): CatalogResource {
  const object = readExact(value, ["product", "name", "shape", "operations", "fields"], ["get_key"]);
  const shape = readString(object.shape);
  if (shape !== "list" && shape !== "singleton") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid catalog shape");
  }
  const operations = readStringList(object.operations, 3, (item) => {
    const operationName = readString(item);
    if (operationName !== "list" && operationName !== "get" && operationName !== "show") {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid catalog operation");
    }
    return operationName;
  }, true);
  if (operations.length < 1) {
    fail(ERROR_KINDS.INVALID_FRAME, "catalog resource must advertise an operation");
  }
  const fields = readArray(object.fields);
  if (fields.length > MAX_RESOURCE_SELECTOR_COUNT) {
    fail(ERROR_KINDS.INVALID_FRAME, "catalog fields exceed their limit");
  }
  const resource: { product: Product; name: string; shape: "list" | "singleton"; operations: ("list" | "get" | "show")[]; get_key?: string; fields: CatalogField[] } = {
    product: product(object.product),
    name: resourceName(object.name),
    shape,
    operations: operations as ("list" | "get" | "show")[],
    fields: fields.map((item) => readCatalogField(item)),
  };
  if (hasOwn(object, "get_key")) {
    resource.get_key = fieldName(object.get_key);
  }
  return resource;
}

function readWireValue(value: unknown): JsonValue {
  if (value === null || typeof value === "boolean" || typeof value === "string" || isWireNumber(value)) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => readWireValue(item));
  }
  if (typeof value === "object" && value !== null) {
    const object = readObject(value);
    const output: JsonObject = Object.create(null) as JsonObject;
    for (const key of Object.keys(object)) {
      validateDynamicKey(key);
      output[key] = readWireValue(object[key]);
    }
    return output;
  }
  fail(ERROR_KINDS.INVALID_FRAME, "invalid dynamic value");
}

function readRecord(value: unknown): WireRecord {
  const object = readObject(value);
  const output: JsonObject = Object.create(null) as JsonObject;
  for (const key of Object.keys(object)) {
    validateDynamicKey(key);
    output[key] = readWireValue(object[key]);
  }
  return output;
}

function readURLClassification(value: unknown): URLClassification {
  const object = readExact(value, ["url", "classifications", "security_alert_classifications", "application"]);
  const classifications = readStringList(object.classifications, MAX_RESOURCE_SELECTOR_COUNT, controlString, false);
  const security = readStringList(object.security_alert_classifications, MAX_RESOURCE_SELECTOR_COUNT, controlString, false);
  return { url: controlString(object.url), classifications, security_alert_classifications: security, application: controlString(object.application) };
}

function readProjectedRecord(value: unknown): ProjectedRecord {
  const object = readExact(value, ["product", "resource", "record"]);
  return { product: product(object.product), resource: resourceName(object.resource), record: readRecord(object.record) };
}

function readDiffIdentity(value: unknown): DiffIdentity {
  const initial = readObject(value);
  const mode = readString(initial.mode);
  if (mode === "get_key") {
    const object = readExact(initial, ["mode", "field"]);
    return { mode: "get_key", field: fieldName(object.field) };
  }
  if (mode === "singleton") {
    readExact(initial, ["mode"]);
    return { mode: "singleton" };
  }
  if (mode === "content_hash") {
    readExact(initial, ["mode"]);
    return { mode: "content_hash" };
  }
  fail(ERROR_KINDS.INVALID_FRAME, "invalid diff identity mode");
}

function readDiffResource(value: unknown): DiffResource {
  const object = readExact(value, ["product", "resource", "identity", "added", "removed", "changed_fields"], ["note"]);
  const result: { product: Product; resource: string; identity: DiffIdentity; added: number; removed: number; changed_fields: number; note?: string } = {
    product: product(object.product), resource: resourceName(object.resource), identity: readDiffIdentity(object.identity),
    added: safeInteger(object.added), removed: safeInteger(object.removed), changed_fields: safeInteger(object.changed_fields),
  };
  if (hasOwn(object, "note")) {
    result.note = controlString(object.note);
  }
  return result;
}

function readDiffRecordRef(value: unknown): DiffRecordRef {
  const object = readObject(value);
  const hasKey = hasOwn(object, "key");
  const hasHash = hasOwn(object, "hash");
  if (hasKey === hasHash) {
    fail(ERROR_KINDS.INVALID_FRAME, "diff record must have exactly one key or hash");
  }
  if (hasKey) {
    const exact = readExact(object, ["product", "resource", "key", "record"]);
    return { product: product(exact.product), resource: resourceName(exact.resource), key: controlString(exact.key), record: readRecord(exact.record) };
  }
  const exact = readExact(object, ["product", "resource", "hash", "record"]);
  return { product: product(exact.product), resource: resourceName(exact.resource), hash: readHex(exact.hash, 64), record: readRecord(exact.record) };
}

function readDiffFieldChange(value: unknown): DiffFieldChange {
  const object = readExact(value, ["product", "resource", "key", "field", "old", "new"]);
  return { product: product(object.product), resource: resourceName(object.resource), key: controlString(object.key), field: fieldName(object.field), old: readWireValue(object.old), new: readWireValue(object.new) };
}

function readItemValue(kind: ItemKind, value: unknown): ItemValue {
  switch (kind) {
    case "catalog_resource": return readCatalogResource(value);
    case "url_classification": return readURLClassification(value);
    case "projected_record": return readProjectedRecord(value);
    case "diff_resource": return readDiffResource(value);
    case "diff_added":
    case "diff_removed": return readDiffRecordRef(value);
    case "diff_field_change": return readDiffFieldChange(value);
    default: fail(ERROR_KINDS.INVALID_FRAME, "unknown item kind");
  }
}

export function decodeItemPayload(kind: ItemKind, data: Uint8Array): ItemValue {
  const checkedKind = readItemKind(kind);
  const object = parseJsonObject(data, {
    maximumBytes: AGGREGATE_ITEM_BYTES,
    maximumDepth: V1_JSON_DEPTH - 1,
  });
  return readItemValue(checkedKind, object);
}

function readSchemaIdentity(value: unknown): SchemaIdentity {
  const object = readExact(value, ["id", "sha256"]);
  const id = readString(object.id);
  const sha256 = readHex(object.sha256, 64);
  if (id !== V1_SCHEMA_ID || sha256 !== V1_SCHEMA_SHA256) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid schema identity");
  }
  return { id, sha256 };
}

function readServerBuild(value: unknown): ServerBuild {
  const object = readExact(value, ["name", "version"]);
  if (readString(object.name) !== "zscalerctl-engine") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid server build name");
  }
  return { name: "zscalerctl-engine", version: structuralString(object.version, 1, 128, MAX_CONTROL_STRING_BYTES) };
}

function readLimits(value: unknown): Limits {
  const object = readExact(value, ["client_frame_bytes", "server_frame_bytes", "json_depth", "aggregate_item_bytes", "fragment_chunk_bytes", "url_count", "read_field_count", "read_filter_count", "product_selector_count", "resource_selector_count", "path_bytes", "control_string_bytes"]);
  const limits = {
    client_frame_bytes: safeInteger(object.client_frame_bytes), server_frame_bytes: safeInteger(object.server_frame_bytes), json_depth: safeInteger(object.json_depth),
    aggregate_item_bytes: safeInteger(object.aggregate_item_bytes), fragment_chunk_bytes: safeInteger(object.fragment_chunk_bytes), url_count: safeInteger(object.url_count),
    read_field_count: safeInteger(object.read_field_count), read_filter_count: safeInteger(object.read_filter_count), product_selector_count: safeInteger(object.product_selector_count),
    resource_selector_count: safeInteger(object.resource_selector_count), path_bytes: safeInteger(object.path_bytes), control_string_bytes: safeInteger(object.control_string_bytes),
  };
  const expected = [V1_FRAME_BYTES, V1_FRAME_BYTES, V1_JSON_DEPTH, AGGREGATE_ITEM_BYTES, FRAGMENT_CHUNK_BYTES, MAX_URL_COUNT, MAX_READ_FIELD_COUNT, MAX_READ_FILTER_COUNT, MAX_PRODUCT_SELECTOR_COUNT, MAX_RESOURCE_SELECTOR_COUNT, MAX_PATH_BYTES, MAX_CONTROL_STRING_BYTES];
  const actual = Object.values(limits);
  for (let index = 0; index < expected.length; index += 1) {
    if (actual[index] !== expected[index]) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid v1 limits");
    }
  }
  return limits;
}

function readEffect(value: unknown): Effect {
  const object = readExact(value, ["kind", "when"]);
  const kind = readString(object.kind);
  const when = readString(object.when);
  if (!["local_filesystem_read", "local_filesystem_write", "local_filesystem_delete", "network_access", "process_execution"].includes(kind) || !["always", "request_dependent", "configuration_dependent"].includes(when)) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid effect");
  }
  return { kind: kind as Effect["kind"], when: when as Effect["when"] };
}

function effectsEqual(actual: readonly Effect[], expected: readonly Effect[]): boolean {
  if (actual.length !== expected.length) return false;
  return actual.every((effect, index) => effect.kind === expected[index].kind && effect.when === expected[index].when);
}

function readEngineCapability(value: unknown): EngineCapability {
  const object = readExact(value, ["name", "operations", "input", "result", "tenant_read_only", "effects"]);
  const name = capability(object.name);
  if (readBoolean(object.tenant_read_only) !== true) {
    fail(ERROR_KINDS.INVALID_FRAME, "engine capability must be tenant read-only");
  }
  const operationValues = readStringList(object.operations, 3, (item) => operation(item), true);
  const effects = readArray(object.effects).map((item) => readEffect(item));
  let expectedOperations: string[];
  let expectedInput: string;
  let expectedResult: string;
  let expectedEffects: Effect[];
  if (name === "engine.manifest") {
    expectedOperations = ["manifest"]; expectedInput = "none"; expectedResult = "engine_manifest"; expectedEffects = [];
  } else if (name === "catalog.schema") {
    expectedOperations = ["list"]; expectedInput = "none"; expectedResult = "resource_catalog"; expectedEffects = [];
  } else if (name === "status.inspect") {
    expectedOperations = ["doctor", "auth_status", "config_status"]; expectedInput = "status"; expectedResult = "status"; expectedEffects = [{ kind: "local_filesystem_read", when: "configuration_dependent" }];
  } else if (name === "zia.url_lookup") {
    expectedOperations = ["lookup"]; expectedInput = "url_lookup"; expectedResult = "url_classifications"; expectedEffects = [
      { kind: "local_filesystem_read", when: "configuration_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" },
    ];
  } else if (name === "resources.read") {
    if (operationValues.length < 1 || operationValues.some((item) => !["list", "get", "show"].includes(item))) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid resource read operations");
    }
    expectedOperations = operationValues; expectedInput = "resource_read"; expectedResult = "projected_records"; expectedEffects = [
      { kind: "local_filesystem_read", when: "configuration_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" },
    ];
  } else if (name === "dump.write") {
    expectedOperations = ["dump"]; expectedInput = "dump"; expectedResult = "dump_summary"; expectedEffects = [
      { kind: "local_filesystem_read", when: "always" }, { kind: "local_filesystem_write", when: "always" }, { kind: "local_filesystem_delete", when: "request_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" },
    ];
  } else {
    expectedOperations = ["diff"]; expectedInput = "diff"; expectedResult = "diff_report"; expectedEffects = [{ kind: "local_filesystem_read", when: "always" }];
  }
  if (operationValues.join("\u0000") !== expectedOperations.join("\u0000") || readString(object.input) !== expectedInput || readString(object.result) !== expectedResult || !effectsEqual(effects, expectedEffects)) {
    fail(ERROR_KINDS.INVALID_FRAME, "engine capability metadata does not match its name");
  }
  return { name, operations: operationValues as Operation[], input: expectedInput, result: expectedResult, tenant_read_only: true, effects };
}

function readEngineManifest(value: unknown): EngineManifest {
  const object = readExact(value, ["version", "tenant_read_only", "capabilities"]);
  if (readString(object.version) !== "engine.v1" || readBoolean(object.tenant_read_only) !== true) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid engine manifest discriminants");
  }
  const capabilities = readArray(object.capabilities);
  if (capabilities.length < 1 || capabilities.length > 64) {
    fail(ERROR_KINDS.INVALID_FRAME, "engine manifest capability count is outside its bound");
  }
  const output: EngineCapability[] = [];
  const seen = new Set<string>();
  for (const value of capabilities) {
    const item = readEngineCapability(value);
    if (seen.has(item.name)) {
      fail(ERROR_KINDS.INVALID_FRAME, "duplicate engine capability");
    }
    seen.add(item.name);
    output.push(item);
  }
  return { version: "engine.v1", tenant_read_only: true, capabilities: output };
}

function readReady(value: unknown): Ready {
  const object = readExact(value, ["type", "protocol", "version", "schema", "server", "limits", "engine"]);
  if (readString(object.type) !== "ready" || readString(object.protocol) !== PROTOCOL || readString(object.version) !== V1_VERSION) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid ready discriminants");
  }
  return { type: "ready", protocol: PROTOCOL, version: V1_VERSION, schema: readSchemaIdentity(object.schema), server: readServerBuild(object.server), limits: readLimits(object.limits), engine: readEngineManifest(object.engine) };
}

function readAuthMode(value: unknown): "oneapi" | "zia-legacy" {
  const text = readString(value);
  if (text !== "oneapi" && text !== "zia-legacy") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid authentication mode");
  }
  return text;
}

function readDoctorStatus(value: unknown): DoctorStatus {
  const object = readExact(value, ["status", "mode", "profile", "config", "auth_mode", "redaction", "timeout", "cache", "proxy", "credentials", "live_api"]);
  return {
    status: controlString(object.status), mode: controlString(object.mode), profile: controlString(object.profile), config: controlString(object.config),
    auth_mode: readAuthMode(object.auth_mode), redaction: redaction(object.redaction), timeout: controlString(object.timeout), cache: controlString(object.cache),
    proxy: controlString(object.proxy), credentials: controlString(object.credentials), live_api: controlString(object.live_api),
  };
}

function readAuthStatus(value: unknown): AuthStatus {
  const object = readExact(value, ["credentials", "credential_exchange", "live_api"]);
  return { credentials: controlString(object.credentials), credential_exchange: controlString(object.credential_exchange), live_api: controlString(object.live_api) };
}

function readConfigCredentials(value: unknown): ConfigCredentials {
  const object = readExact(value, ["client_id_set", "client_secret_set", "client_secret_file_set"], ["client_secret_scheme"]);
  const result: { client_id_set: boolean; client_secret_set: boolean; client_secret_file_set: boolean; client_secret_scheme?: string } = {
    client_id_set: readBoolean(object.client_id_set), client_secret_set: readBoolean(object.client_secret_set), client_secret_file_set: readBoolean(object.client_secret_file_set),
  };
  if (hasOwn(object, "client_secret_scheme")) result.client_secret_scheme = controlString(object.client_secret_scheme);
  return result;
}

function readConfigZPA(value: unknown): ConfigZPA {
  const object = readExact(value, ["customer_id_set", "microtenant_id_set"]);
  return { customer_id_set: readBoolean(object.customer_id_set), microtenant_id_set: readBoolean(object.microtenant_id_set) };
}

function readConfigZIALegacy(value: unknown): ConfigZIALegacy {
  const object = readExact(value, ["username_set", "password_set", "password_file_set", "api_key_set", "api_key_file_set", "cloud_set"], ["password_scheme", "api_key_scheme"]);
  const result: { username_set: boolean; password_set: boolean; password_file_set: boolean; password_scheme?: string; api_key_set: boolean; api_key_file_set: boolean; api_key_scheme?: string; cloud_set: boolean } = {
    username_set: readBoolean(object.username_set), password_set: readBoolean(object.password_set), password_file_set: readBoolean(object.password_file_set),
    api_key_set: readBoolean(object.api_key_set), api_key_file_set: readBoolean(object.api_key_file_set), cloud_set: readBoolean(object.cloud_set),
  };
  if (hasOwn(object, "password_scheme")) result.password_scheme = controlString(object.password_scheme);
  if (hasOwn(object, "api_key_scheme")) result.api_key_scheme = controlString(object.api_key_scheme);
  return result;
}

function readConfigProxy(value: unknown): ConfigProxy {
  const object = readExact(value, ["url_set", "from_environment"]);
  return { url_set: readBoolean(object.url_set), from_environment: readBoolean(object.from_environment) };
}

function readConfigDefaults(value: unknown): ConfigDefaults {
  const object = readExact(value, ["redaction", "no_cache"]);
  return { redaction: redaction(object.redaction), no_cache: readBoolean(object.no_cache) };
}

function readConfigStatus(value: unknown): ConfigStatus {
  const object = readExact(value, ["source", "config_file_set", "profile", "auth_mode", "vanity_domain_set", "credentials", "zpa", "zia_legacy", "proxy", "defaults"], ["cloud"]);
  const result: { source: string; config_file_set: boolean; profile: string; auth_mode: "oneapi" | "zia-legacy"; vanity_domain_set: boolean; cloud?: string; credentials: ConfigCredentials; zpa: ConfigZPA; zia_legacy: ConfigZIALegacy; proxy: ConfigProxy; defaults: ConfigDefaults } = {
    source: controlString(object.source), config_file_set: readBoolean(object.config_file_set), profile: controlString(object.profile), auth_mode: readAuthMode(object.auth_mode), vanity_domain_set: readBoolean(object.vanity_domain_set),
    credentials: readConfigCredentials(object.credentials), zpa: readConfigZPA(object.zpa), zia_legacy: readConfigZIALegacy(object.zia_legacy), proxy: readConfigProxy(object.proxy), defaults: readConfigDefaults(object.defaults),
  };
  if (hasOwn(object, "cloud")) result.cloud = controlString(object.cloud);
  return result;
}

function readDumpFailure(value: unknown): DumpFailure {
  const object = readExact(value, ["product", "resource", "phase", "kind"]);
  const phase = readString(object.phase);
  const kind = readString(object.kind);
  const valid = phase === "list" && kind === "list_failed" || phase === "show" && kind === "show_failed" || phase === "project" && kind === "projection_failed" || phase === "validate" && kind === "subset_failed";
  if (!valid) fail(ERROR_KINDS.INVALID_FRAME, "invalid dump failure phase/kind");
  return { product: product(object.product), resource: resourceName(object.resource), phase, kind } as DumpFailure;
}

export function decodeCanonicalBase64(value: string): Uint8Array {
  if (typeof value !== "string") {
    fail(ERROR_KINDS.INVALID_FRAME, "base64 data must be a string");
  }
  const text = value;
  if (text.length < 4 || text.length % 4 !== 0) {
    fail(ERROR_KINDS.INVALID_FRAME, "base64 data is not canonical");
  }
  let padding = 0;
  if (text.endsWith("=")) padding += 1;
  if (text.endsWith("==")) padding += 1;
  const contentLength = text.length - padding;
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index);
    const alpha = code >= 0x41 && code <= 0x5a || code >= 0x61 && code <= 0x7a || code >= 0x30 && code <= 0x39;
    if (index < contentLength) {
      if (!alpha && text[index] !== "+" && text[index] !== "/") {
        fail(ERROR_KINDS.INVALID_FRAME, "base64 data is not canonical");
      }
    } else if (text[index] !== "=") {
      fail(ERROR_KINDS.INVALID_FRAME, "base64 padding is not canonical");
    }
  }
  const values = (character: string): number => {
    const code = character.charCodeAt(0);
    if (code >= 0x41 && code <= 0x5a) return code - 0x41;
    if (code >= 0x61 && code <= 0x7a) return code - 0x61 + 26;
    if (code >= 0x30 && code <= 0x39) return code - 0x30 + 52;
    if (character === "+") return 62;
    if (character === "/") return 63;
    return 0;
  };
  const outputLength = text.length / 4 * 3 - padding;
  if (outputLength < 1 || outputLength > FRAGMENT_CHUNK_BYTES) {
    fail(ERROR_KINDS.INVALID_FRAME, "base64 data length is outside its bound");
  }
  const output = new Uint8Array(outputLength);
  let outputIndex = 0;
  for (let index = 0; index < text.length; index += 4) {
    const first = values(text[index]);
    const second = values(text[index + 1]);
    const third = text[index + 2] === "=" ? 0 : values(text[index + 2]);
    const fourth = text[index + 3] === "=" ? 0 : values(text[index + 3]);
    if (text[index + 2] === "=" && (second & 0x0f) !== 0) {
      fail(ERROR_KINDS.INVALID_FRAME, "base64 trailing bits are not zero");
    }
    if (text[index + 3] === "=" && text[index + 2] !== "=" && (third & 0x03) !== 0) {
      fail(ERROR_KINDS.INVALID_FRAME, "base64 trailing bits are not zero");
    }
    if (outputIndex < output.length) output[outputIndex++] = (first << 2) | (second >> 4);
    if (outputIndex < output.length) output[outputIndex++] = ((second & 0x0f) << 4) | (third >> 2);
    if (outputIndex < output.length) output[outputIndex++] = ((third & 0x03) << 6) | fourth;
  }
  return output;
}

function base64Value(value: unknown): string {
  const text = readString(value);
  decodeCanonicalBase64(text);
  return text;
}

function readItemFrame(value: unknown): ItemFrame {
  const object = readExact(value, ["type", "id", "seq", "kind", "item"]);
  if (readString(object.type) !== "item") fail(ERROR_KINDS.INVALID_FRAME, "invalid item type");
  const id = safeInteger(object.id, true);
  const seq = safeInteger(object.seq, true);
  const kind = readItemKind(object.kind);
  return { type: "item", id, seq, kind, item: readItemValue(kind, object.item) };
}

function readItemBegin(value: unknown): ItemBegin {
  const object = readExact(value, ["type", "id", "seq", "item_id", "kind", "encoding", "bytes"]);
  if (readString(object.type) !== "item_begin" || readString(object.encoding) !== "json") {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid item-begin discriminants");
  }
  const bytes = safeInteger(object.bytes);
  if (bytes < 2 || bytes > AGGREGATE_ITEM_BYTES) fail(ERROR_KINDS.INVALID_FRAME, "item byte count is outside its bound");
  return { type: "item_begin", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), item_id: safeInteger(object.item_id, true), kind: readItemKind(object.kind), encoding: "json", bytes };
}

function readItemChunk(value: unknown): ItemChunk {
  const object = readExact(value, ["type", "id", "seq", "item_id", "index", "data"]);
  if (readString(object.type) !== "item_chunk") fail(ERROR_KINDS.INVALID_FRAME, "invalid item-chunk type");
  const index = safeInteger(object.index);
  if (index > 127) fail(ERROR_KINDS.INVALID_FRAME, "chunk index is outside its bound");
  const data = base64Value(object.data);
  return { type: "item_chunk", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), item_id: safeInteger(object.item_id, true), index, data };
}

function readItemEnd(value: unknown): ItemEnd {
  const object = readExact(value, ["type", "id", "seq", "item_id", "chunks", "sha256"]);
  if (readString(object.type) !== "item_end") fail(ERROR_KINDS.INVALID_FRAME, "invalid item-end type");
  const chunks = safeInteger(object.chunks);
  if (chunks < 1 || chunks > 128) fail(ERROR_KINDS.INVALID_FRAME, "chunk count is outside its bound");
  return { type: "item_end", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), item_id: safeInteger(object.item_id, true), chunks, sha256: readHex(object.sha256, 64) };
}

function readProgress(value: unknown): Progress {
  const object = readExact(value, ["type", "id", "seq", "phase", "current", "total", "product", "resource"]);
  if (readString(object.type) !== "progress" || readString(object.phase) !== "resource_started") fail(ERROR_KINDS.INVALID_FRAME, "invalid progress discriminants");
  const current = safeInteger(object.current, true);
  const total = safeInteger(object.total, true);
  if (current > total) fail(ERROR_KINDS.INVALID_FRAME, "progress current exceeds total");
  return { type: "progress", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), phase: "resource_started", current, total, product: product(object.product), resource: resourceName(object.resource) };
}

function readWarning(value: unknown): Warning {
  const object = readExact(value, ["type", "id", "seq", "warning"]);
  if (readString(object.type) !== "warning") fail(ERROR_KINDS.INVALID_FRAME, "invalid warning type");
  return { type: "warning", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), warning: readDumpFailure(object.warning) };
}

function readDumpSide(value: unknown, expectedSide: "old" | "new"): { side: "old" | "new"; manifest_schema: string; redaction: Redaction; status: "complete" | "partial"; partial: boolean } {
  const object = readExact(value, ["side", "manifest_schema", "redaction", "status", "partial"]);
  const side = readString(object.side);
  const status = readString(object.status);
  const partial = readBoolean(object.partial);
  if (side !== expectedSide || (status !== "complete" && status !== "partial") || partial !== (status === "partial")) {
    fail(ERROR_KINDS.INVALID_FRAME, "invalid dump-side reference");
  }
  return { side: expectedSide, manifest_schema: controlString(object.manifest_schema), redaction: redaction(object.redaction), status, partial };
}

function readDiffCounts(value: unknown): DiffCounts {
  const object = readExact(value, ["resources_compared", "resources_with_drift", "records_added", "records_removed", "records_changed"]);
  const result = { resources_compared: safeInteger(object.resources_compared), resources_with_drift: safeInteger(object.resources_with_drift), records_added: safeInteger(object.records_added), records_removed: safeInteger(object.records_removed), records_changed: safeInteger(object.records_changed) };
  if (result.resources_with_drift > result.resources_compared) fail(ERROR_KINDS.INVALID_FRAME, "resources with drift exceeds resources compared");
  return result;
}

function readCompletionResult(value: unknown): CompletionResult {
  const object = readObject(value);
  const kind = readString(object.kind);
  if (kind === "engine_manifest") {
    const exact = readExact(object, ["kind", "manifest"]);
    return { kind, manifest: readEngineManifest(exact.manifest) };
  }
  if (kind === "catalog_summary") {
    const exact = readExact(object, ["kind", "resources", "stream_items_emitted"]);
    const resources = safeInteger(exact.resources); const emitted = safeInteger(exact.stream_items_emitted);
    if (resources !== emitted) fail(ERROR_KINDS.INVALID_FRAME, "catalog summary counters do not match");
    return { kind, resources, stream_items_emitted: emitted };
  }
  if (kind === "doctor_status") {
    const exact = readExact(object, ["kind", "status"]);
    return { kind, status: readDoctorStatus(exact.status) };
  }
  if (kind === "auth_status") {
    const exact = readExact(object, ["kind", "status"]);
    return { kind, status: readAuthStatus(exact.status) };
  }
  if (kind === "config_status") {
    const exact = readExact(object, ["kind", "status"]);
    return { kind, status: readConfigStatus(exact.status) };
  }
  if (kind === "url_lookup_summary") {
    const exact = readExact(object, ["kind", "classifications", "stream_items_emitted"]);
    const classifications = safeInteger(exact.classifications); const emitted = safeInteger(exact.stream_items_emitted);
    if (classifications !== emitted) fail(ERROR_KINDS.INVALID_FRAME, "URL summary counters do not match");
    return { kind, classifications, stream_items_emitted: emitted };
  }
  if (kind === "resource_read_summary") {
    const exact = readExact(object, ["kind", "records", "stream_items_emitted"]);
    const records = safeInteger(exact.records); const emitted = safeInteger(exact.stream_items_emitted);
    if (records !== emitted) fail(ERROR_KINDS.INVALID_FRAME, "resource summary counters do not match");
    return { kind, records, stream_items_emitted: emitted };
  }
  if (kind === "dump_summary") {
    const exact = readExact(object, ["kind", "records_written", "resources_written", "warning_count", "partial", "redaction", "failures", "stream_items_emitted"]);
    const failures = readArray(exact.failures);
    if (failures.length > MAX_RESOURCE_SELECTOR_COUNT) fail(ERROR_KINDS.INVALID_FRAME, "dump failures exceed their limit");
    const warningCount = safeInteger(exact.warning_count);
    if (warningCount !== failures.length || readBoolean(exact.partial) !== (failures.length > 0) || safeInteger(exact.stream_items_emitted) !== 0) {
      fail(ERROR_KINDS.INVALID_FRAME, "invalid dump summary counters");
    }
    return { kind, records_written: safeInteger(exact.records_written), resources_written: safeInteger(exact.resources_written), warning_count: warningCount, partial: failures.length > 0, redaction: redaction(exact.redaction), failures: failures.map((item) => readDumpFailure(item)), stream_items_emitted: 0 };
  }
  if (kind === "diff_summary") {
    const exact = readExact(object, ["kind", "schema", "old", "new", "summary", "has_drift", "stream_items_emitted"]);
    if (readString(exact.schema) !== "zscalerctl.diff.v1") fail(ERROR_KINDS.INVALID_FRAME, "invalid diff summary schema");
    const summary = readDiffCounts(exact.summary);
    const hasDrift = summary.resources_with_drift > 0 || summary.records_added > 0 || summary.records_removed > 0 || summary.records_changed > 0;
    if (readBoolean(exact.has_drift) !== hasDrift) fail(ERROR_KINDS.INVALID_FRAME, "diff has_drift does not match summary");
    return { kind, schema: "zscalerctl.diff.v1", old: readDumpSide(exact.old, "old") as DiffSummary["old"], new: readDumpSide(exact.new, "new") as DiffSummary["new"], summary, has_drift: hasDrift, stream_items_emitted: safeInteger(exact.stream_items_emitted) };
  }
  fail(ERROR_KINDS.INVALID_FRAME, "unknown completion result kind");
}

function readCompleted(value: unknown): Completed {
  const object = readExact(value, ["type", "id", "seq", "result"]);
  if (readString(object.type) !== "completed") fail(ERROR_KINDS.INVALID_FRAME, "invalid completed type");
  return { type: "completed", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), result: readCompletionResult(object.result) };
}

function readOperationFailure(value: unknown): OperationFailure {
  const object = readObject(value);
  const kind = readString(object.kind);
  if (kind === "missing_credentials") {
    const exact = readExact(object, ["kind"], ["missing"]);
    let missing: string[] | undefined;
    if (hasOwn(exact, "missing")) {
      missing = readStringList(exact.missing, 32, (item) => {
        const name = readString(item);
        if (!MISSING_CREDENTIAL_NAMES.has(name)) fail(ERROR_KINDS.INVALID_FRAME, "invalid missing credential name");
        return name;
      }, true);
    }
    return missing === undefined ? { kind: "missing_credentials" } : { kind: "missing_credentials", missing };
  }
  const exact = readExact(object, ["kind"]);
  const failureKind = readString(exact.kind);
  if (!FAILURE_KINDS.has(failureKind)) fail(ERROR_KINDS.INVALID_FRAME, "invalid operation failure kind");
  return { kind: failureKind as FailureKind };
}

function readFailed(value: unknown): Failed {
  const object = readExact(value, ["type", "id", "seq", "error"]);
  if (readString(object.type) !== "failed") fail(ERROR_KINDS.INVALID_FRAME, "invalid failed type");
  return { type: "failed", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), error: readOperationFailure(object.error) };
}

function readCanceled(value: unknown): Canceled {
  const object = readExact(value, ["type", "id", "seq", "error"]);
  if (readString(object.type) !== "canceled") fail(ERROR_KINDS.INVALID_FRAME, "invalid canceled type");
  const error = readExact(object.error, ["kind"]);
  if (readString(error.kind) !== "canceled") fail(ERROR_KINDS.INVALID_FRAME, "invalid canceled error");
  return { type: "canceled", id: safeInteger(object.id, true), seq: safeInteger(object.seq, true), error: { kind: "canceled" } };
}

function readProtocolErrorFrame(value: unknown): BootstrapProtocolError {
  const object = readExact(value, ["type", "fatal", "error"]);
  if (readString(object.type) !== "protocol_error" || readBoolean(object.fatal) !== true) fail(ERROR_KINDS.INVALID_FRAME, "invalid v1 protocol error");
  return { type: "protocol_error", fatal: true, error: readProtocolError(object.error) };
}

export function decodeServerFrame(data: Uint8Array): import("./types.ts").ServerFrame {
  const object = parseJsonObject(data, { maximumBytes: V1_FRAME_BYTES, maximumDepth: V1_JSON_DEPTH });
  const type = readString(object.type);
  switch (type) {
    case "ready": return readReady(object);
    case "request_rejected": {
      const frame = readExact(object, ["type", "id", "reason"]);
      if (readString(frame.reason) !== "busy") fail(ERROR_KINDS.INVALID_FRAME, "invalid request rejection");
      return { type: "request_rejected", id: safeInteger(frame.id, true), reason: "busy" };
    }
    case "started": {
      const frame = readExact(object, ["type", "id", "seq", "capability", "operation"]);
      const pair = validateCapabilityOperation(frame.capability, frame.operation);
      return { type: "started", id: safeInteger(frame.id, true), seq: safeInteger(frame.seq, true), ...pair };
    }
    case "item": return readItemFrame(object);
    case "item_begin": return readItemBegin(object);
    case "item_chunk": return readItemChunk(object);
    case "item_end": return readItemEnd(object);
    case "progress": return readProgress(object);
    case "warning": return readWarning(object);
    case "completed": return readCompleted(object);
    case "failed": return readFailed(object);
    case "canceled": return readCanceled(object);
    case "protocol_error": return readProtocolErrorFrame(object);
    case "request":
    case "cancel": fail(ERROR_KINDS.WRONG_DIRECTION, "client frame sent to server");
    default: fail(ERROR_KINDS.INVALID_FRAME, "unknown server frame type");
  }
}

export const decodeV1ServerFrame = decodeServerFrame;

const FIELD_ORDERS: readonly (readonly string[])[] = [
  ["type", "protocol", "versions", "bootstrap"], ["type", "protocol", "version"], ["type", "protocol", "reason"], ["type", "fatal", "error"],
  ["frame_bytes", "json_depth"], ["type", "protocol", "version", "schema", "server", "limits", "engine"], ["id", "sha256"], ["name", "version"],
  ["client_frame_bytes", "server_frame_bytes", "json_depth", "aggregate_item_bytes", "fragment_chunk_bytes", "url_count", "read_field_count", "read_filter_count", "product_selector_count", "resource_selector_count", "path_bytes", "control_string_bytes"],
  ["version", "tenant_read_only", "capabilities"], ["name", "operations", "input", "result", "tenant_read_only", "effects"], ["kind", "when"],
  ["type", "id", "capability", "operation", "input"], ["type", "id", "capability", "operation"], ["urls"],
  ["product", "resource", "fields", "filters", "search"], ["product", "resource", "record_id", "fields"], ["product", "resource", "fields"],
  ["field", "operator", "value"], ["output_dir", "products", "resources", "continue_on_error", "force"], ["old_dir", "new_dir", "products", "resources", "ignore_operational", "allow_partial"],
  ["product", "resource"], ["type", "id"], ["type", "id", "reason"], ["type", "id", "seq", "capability", "operation"],
  ["type", "id", "seq", "kind", "item"], ["type", "id", "seq", "item_id", "kind", "encoding", "bytes"], ["type", "id", "seq", "item_id", "index", "data"], ["type", "id", "seq", "item_id", "chunks", "sha256"],
  ["type", "id", "seq", "phase", "current", "total", "product", "resource"], ["type", "id", "seq", "warning"],
  ["name", "json_name", "classification", "allowed_modes", "fields"], ["product", "name", "shape", "operations", "get_key", "fields"],
  ["url", "classifications", "security_alert_classifications", "application"], ["product", "resource", "record"], ["mode", "field"], ["product", "resource", "identity", "added", "removed", "changed_fields", "note"],
  ["product", "resource", "key", "record"], ["product", "resource", "hash", "record"], ["product", "resource", "key", "field", "old", "new"],
  ["kind", "manifest"], ["kind", "resources", "stream_items_emitted"], ["kind", "status"], ["kind", "classifications", "stream_items_emitted"], ["kind", "records", "stream_items_emitted"],
  ["kind", "records_written", "resources_written", "warning_count", "partial", "redaction", "failures", "stream_items_emitted"], ["side", "manifest_schema", "redaction", "status", "partial"],
  ["resources_compared", "resources_with_drift", "records_added", "records_removed", "records_changed"], ["kind", "schema", "old", "new", "summary", "has_drift", "stream_items_emitted"],
  ["status", "mode", "profile", "config", "auth_mode", "redaction", "timeout", "cache", "proxy", "credentials", "live_api"], ["credentials", "credential_exchange", "live_api"],
  ["source", "config_file_set", "profile", "auth_mode", "vanity_domain_set", "cloud", "credentials", "zpa", "zia_legacy", "proxy", "defaults"],
  ["client_id_set", "client_secret_set", "client_secret_file_set", "client_secret_scheme"], ["customer_id_set", "microtenant_id_set"],
  ["username_set", "password_set", "password_file_set", "password_scheme", "api_key_set", "api_key_file_set", "api_key_scheme", "cloud_set"], ["url_set", "from_environment"], ["redaction", "no_cache"],
  ["kind", "missing"], ["kind"], ["type", "id", "seq", "result"], ["type", "id", "seq", "error"],
];

function compareText(left: string, right: string): number {
  const leftBytes = new TextEncoder().encode(left);
  const rightBytes = new TextEncoder().encode(right);
  const length = Math.min(leftBytes.length, rightBytes.length);
  for (let index = 0; index < length; index += 1) {
    if (leftBytes[index] !== rightBytes[index]) return leftBytes[index] - rightBytes[index];
  }
  return leftBytes.length - rightBytes.length;
}

function canonicalKeys(object: UnknownObject, dynamic: boolean): string[] {
  const keys = Object.keys(object);
  if (dynamic) return keys.sort(compareText);
  for (const order of FIELD_ORDERS) {
    if (keys.every((key) => order.includes(key))) {
      return order.filter((key) => hasOwn(object, key));
    }
  }
  return keys;
}

function isDiffFieldChangeObject(object: UnknownObject): boolean {
  return hasOwn(object, "old") && hasOwn(object, "new") && hasOwn(object, "field") && hasOwn(object, "key");
}

function typedNode(
  value: unknown,
  dynamic = false,
  depth = 1,
  ancestors: Set<object> = new Set(),
): JsonNode {
  if (value === null || typeof value === "boolean" || typeof value === "string" || isWireNumber(value)) {
    if (dynamic && typeof value === "number") fail(ERROR_KINDS.INVALID_FRAME, "dynamic numbers must be WireNumber values");
    return value;
  }
  if (typeof value === "number") {
    if (dynamic) fail(ERROR_KINDS.INVALID_FRAME, "dynamic numbers must be WireNumber values");
    return structuralNumber(typedInteger(value));
  }
  if (Array.isArray(value)) {
    if (depth > V1_JSON_DEPTH || ancestors.has(value)) {
      fail(ERROR_KINDS.INVALID_FRAME, "typed JSON nesting or cycle is invalid");
    }
    ancestors.add(value);
    try {
      return value.map((item) => typedNode(item, dynamic, depth + 1, ancestors));
    } finally {
      ancestors.delete(value);
    }
  }
  if (typeof value === "object" && value !== null) {
    if (depth > V1_JSON_DEPTH || ancestors.has(value)) {
      fail(ERROR_KINDS.INVALID_FRAME, "typed JSON nesting or cycle is invalid");
    }
    ancestors.add(value);
    const object = value as UnknownObject;
    const diffFieldChange = isDiffFieldChangeObject(object);
    const entries: Entry[] = [];
    try {
      for (const key of canonicalKeys(object, dynamic)) {
        const childDynamic = dynamic || key === "record" || (diffFieldChange && (key === "old" || key === "new"));
        entries.push(entry(key, typedNode(object[key], childDynamic, depth + 1, ancestors)));
      }
      return orderedJson(entries);
    } finally {
      ancestors.delete(value);
    }
  }
  fail(ERROR_KINDS.INVALID_FRAME, "unsupported typed JSON value");
}

export function encodeServerFrame(frame: import("./types.ts").ServerFrame): Uint8Array {
  const node = typedNode(frame);
  const data = encodeOrderedJson((node as { entries: readonly Entry[] }).entries, { maximumBytes: V1_FRAME_BYTES, maximumDepth: V1_JSON_DEPTH });
  decodeServerFrame(data);
  return data;
}

export const encodeV1ServerFrame = encodeServerFrame;
