export const PROTOCOL = "zscalerctl.engine.stdio" as const;
export const V1_VERSION = "1" as const;

export const BOOTSTRAP_SCHEMA_ID = "urn:zscalerctl:engine-stdio:bootstrap:1" as const;
export const BOOTSTRAP_SCHEMA_SHA256 = "e559b5568159568978b8b6fdaef9a75cd4022d2cefd1895338e890b7633720cf" as const;
export const V1_SCHEMA_ID = "urn:zscalerctl:engine-stdio:protocol:1" as const;
export const V1_SCHEMA_SHA256 = "6cba5a8170e538bd6eacde38c84526873f691421d6dc5f57cacfbd5f9438c522" as const;

export const BOOTSTRAP_FRAME_BYTES = 64 * 1024;
export const BOOTSTRAP_JSON_DEPTH = 8;
export const V1_FRAME_BYTES = 1 * 1024 * 1024;
export const V1_JSON_DEPTH = 64;
export const AGGREGATE_ITEM_BYTES = 64 * 1024 * 1024;
export const FRAGMENT_CHUNK_BYTES = 512 * 1024;
export const MAX_SAFE_INTEGER = 2 ** 53 - 1;

export const MAX_URL_COUNT = 1024;
export const MAX_READ_FIELD_COUNT = 1024;
export const MAX_READ_FILTER_COUNT = 1024;
export const MAX_PRODUCT_SELECTOR_COUNT = 16;
export const MAX_RESOURCE_SELECTOR_COUNT = 4096;
export const MAX_PATH_BYTES = 32768;
export const MAX_CONTROL_STRING_BYTES = 8192;
