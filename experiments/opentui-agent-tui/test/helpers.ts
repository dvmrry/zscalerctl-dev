import type {
  AuthStatusResponse,
  CatalogResponse,
  ConfigStatusResponse,
  DiffResponse,
  DoctorResponse,
  ManifestResponse,
  Ready,
  ResourceReadResponse,
  URLLookupResponse
} from "../../../clients/typescript/src/index.ts";

import type {EnginePort} from "../src/zscalerctl/adapter.ts";

export const READY: Ready = {
  type: "ready",
  protocol: "zscalerctl.engine.stdio",
  version: "1",
  schema: {id: "urn:zscalerctl:engine-stdio:protocol:1", sha256: "a".repeat(64)},
  server: {name: "zscalerctl-engine", version: "test"},
  limits: {
    client_frame_bytes: 1_048_576,
    server_frame_bytes: 1_048_576,
    json_depth: 64,
    aggregate_item_bytes: 67_108_864,
    fragment_chunk_bytes: 524_288,
    url_count: 1024,
    read_field_count: 1024,
    read_filter_count: 1024,
    product_selector_count: 16,
    resource_selector_count: 4096,
    path_bytes: 32_768,
    control_string_bytes: 8_192
  },
  engine: {version: "engine.v1", tenant_read_only: true, capabilities: []}
};

export const MANIFEST_RESPONSE: ManifestResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {kind: "engine_manifest", manifest: READY.engine}
};

export const CATALOG_RESPONSE: CatalogResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {kind: "catalog_summary", resources: 0, stream_items_emitted: 0}
};

const DOCTOR_RESPONSE: DoctorResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {
    kind: "doctor_status",
    status: {
      status: "ok",
      mode: "environment",
      profile: "none",
      config: "none",
      auth_mode: "oneapi",
      redaction: "standard",
      timeout: "30s",
      cache: "enabled",
      proxy: "not configured",
      credentials: "configured",
      live_api: "not contacted"
    }
  }
};

const AUTH_RESPONSE: AuthStatusResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {kind: "auth_status", status: {credentials: "configured", credential_exchange: "not attempted", live_api: "not contacted"}}
};

const CONFIG_RESPONSE: ConfigStatusResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {
    kind: "config_status",
    status: {
      source: "environment",
      config_file_set: false,
      profile: "none",
      auth_mode: "oneapi",
      vanity_domain_set: true,
      credentials: {client_id_set: true, client_secret_set: true, client_secret_file_set: false},
      zpa: {customer_id_set: false, microtenant_id_set: false},
      zia_legacy: {
        username_set: false,
        password_set: false,
        password_file_set: false,
        api_key_set: false,
        api_key_file_set: false,
        cloud_set: false
      },
      proxy: {url_set: false, from_environment: false},
      defaults: {redaction: "standard", no_cache: false}
    }
  }
};

const LOOKUP_RESPONSE: URLLookupResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {kind: "url_lookup_summary", classifications: 0, stream_items_emitted: 0}
};

export const READ_RESPONSE: ResourceReadResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {kind: "resource_read_summary", records: 0, stream_items_emitted: 0}
};

const DIFF_RESPONSE: DiffResponse = {
  id: 1,
  items: [],
  progress: [],
  warnings: [],
  result: {
    kind: "diff_summary",
    schema: "zscalerctl.diff.v1",
    old: {side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false},
    new: {side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false},
    summary: {resources_compared: 0, resources_with_drift: 0, records_added: 0, records_removed: 0, records_changed: 0},
    has_drift: false,
    stream_items_emitted: 0
  }
};

export function fakeEngine(overrides: Partial<EnginePort> = {}): EnginePort {
  const base: EnginePort = {
    ready: READY,
    manifest: async () => MANIFEST_RESPONSE,
    catalog: async () => CATALOG_RESPONSE,
    doctor: async () => DOCTOR_RESPONSE,
    authStatus: async () => AUTH_RESPONSE,
    configStatus: async () => CONFIG_RESPONSE,
    lookup: async () => LOOKUP_RESPONSE,
    list: async () => READ_RESPONSE,
    get: async () => READ_RESPONSE,
    show: async () => READ_RESPONSE,
    diff: async () => DIFF_RESPONSE,
    close: async () => undefined
  };
  return {...base, ...overrides};
}
