import type { JsonObject, JsonValue, WireNumber } from "./json.ts";

export type SafeInteger = number;
export type Product = "zia" | "zpa" | "ztw" | "zcc" | "zidentity";
export type Redaction = "standard" | "share" | "paranoid";
export type Capability =
  | "engine.manifest"
  | "catalog.schema"
  | "status.inspect"
  | "zia.url_lookup"
  | "resources.read"
  | "dump.write"
  | "diff.compare";
export type Operation =
  | "manifest"
  | "list"
  | "get"
  | "show"
  | "doctor"
  | "auth_status"
  | "config_status"
  | "lookup"
  | "dump"
  | "diff";

export interface BootstrapLimits {
  readonly frame_bytes: SafeInteger;
  readonly json_depth: SafeInteger;
}

export interface Hello {
  readonly type: "hello";
  readonly protocol: string;
  readonly versions: readonly string[];
  readonly bootstrap: BootstrapLimits;
}

export interface Initialize {
  readonly type: "initialize";
  readonly protocol: string;
  readonly version: string;
}

export interface Reject {
  readonly type: "reject";
  readonly protocol: string;
  readonly reason: "unsupported_protocol";
}

export type ProtocolErrorKind = "protocol_violation" | "unsupported_protocol" | "frame_too_large" | "internal";

export interface BootstrapError {
  readonly kind: ProtocolErrorKind;
}

export interface BootstrapProtocolError {
  readonly type: "protocol_error";
  readonly fatal: true;
  readonly error: BootstrapError;
}

export type BootstrapClientFrame = Initialize | Reject;
export type BootstrapServerFrame = Hello | BootstrapProtocolError;

export interface SchemaIdentity {
  readonly id: string;
  readonly sha256: string;
}

export interface ServerBuild {
  readonly name: "zscalerctl-engine";
  readonly version: string;
}

export interface Limits {
  readonly client_frame_bytes: number;
  readonly server_frame_bytes: number;
  readonly json_depth: number;
  readonly aggregate_item_bytes: number;
  readonly fragment_chunk_bytes: number;
  readonly url_count: number;
  readonly read_field_count: number;
  readonly read_filter_count: number;
  readonly product_selector_count: number;
  readonly resource_selector_count: number;
  readonly path_bytes: number;
  readonly control_string_bytes: number;
}

export type EffectKind = "local_filesystem_read" | "local_filesystem_write" | "local_filesystem_delete" | "network_access" | "process_execution";
export type EffectWhen = "always" | "request_dependent" | "configuration_dependent";

export interface Effect {
  readonly kind: EffectKind;
  readonly when: EffectWhen;
}

export interface EngineCapability {
  readonly name: Capability;
  readonly operations: readonly Operation[];
  readonly input: string;
  readonly result: string;
  readonly tenant_read_only: true;
  readonly effects: readonly Effect[];
}

export interface EngineManifest {
  readonly version: "engine.v1";
  readonly tenant_read_only: true;
  readonly capabilities: readonly EngineCapability[];
}

export interface Ready {
  readonly type: "ready";
  readonly protocol: string;
  readonly version: "1";
  readonly schema: SchemaIdentity;
  readonly server: ServerBuild;
  readonly limits: Limits;
  readonly engine: EngineManifest;
}

export interface ManifestRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "engine.manifest";
  readonly operation: "manifest";
}

export interface CatalogRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "catalog.schema";
  readonly operation: "list";
}

export interface DoctorRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "status.inspect";
  readonly operation: "doctor";
}

export interface AuthStatusRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "status.inspect";
  readonly operation: "auth_status";
}

export interface ConfigStatusRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "status.inspect";
  readonly operation: "config_status";
}

export interface URLLookupInput {
  readonly urls: readonly string[];
}

export interface URLLookupRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "zia.url_lookup";
  readonly operation: "lookup";
  readonly input: URLLookupInput;
}

export type FilterOperator = "exact" | "contains";

export interface Filter {
  readonly field: string;
  readonly operator: FilterOperator;
  readonly value: string;
}

export interface ResourceListInput {
  readonly product: Product;
  readonly resource: string;
  readonly fields: readonly string[];
  readonly filters: readonly Filter[];
  readonly search: string;
}

export interface ResourceListRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "resources.read";
  readonly operation: "list";
  readonly input: ResourceListInput;
}

export interface ResourceGetInput {
  readonly product: Product;
  readonly resource: string;
  readonly record_id: string;
  readonly fields: readonly string[];
}

export interface ResourceGetRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "resources.read";
  readonly operation: "get";
  readonly input: ResourceGetInput;
}

export interface ResourceShowInput {
  readonly product: Product;
  readonly resource: string;
  readonly fields: readonly string[];
}

export interface ResourceShowRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "resources.read";
  readonly operation: "show";
  readonly input: ResourceShowInput;
}

export interface ResourceSelector {
  readonly product: Product;
  readonly resource: string;
}

export interface DumpInput {
  readonly output_dir: string;
  readonly products: readonly Product[];
  readonly resources: readonly ResourceSelector[];
  readonly continue_on_error: boolean;
  readonly force: boolean;
}

export interface DumpRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "dump.write";
  readonly operation: "dump";
  readonly input: DumpInput;
}

export interface DiffInput {
  readonly old_dir: string;
  readonly new_dir: string;
  readonly products: readonly Product[];
  readonly resources: readonly ResourceSelector[];
  readonly ignore_operational: boolean;
  readonly allow_partial: boolean;
}

export interface DiffRequest {
  readonly type: "request";
  readonly id: SafeInteger;
  readonly capability: "diff.compare";
  readonly operation: "diff";
  readonly input: DiffInput;
}

export interface Cancel {
  readonly type: "cancel";
  readonly id: SafeInteger;
}

export type ClientRequest =
  | ManifestRequest
  | CatalogRequest
  | DoctorRequest
  | AuthStatusRequest
  | ConfigStatusRequest
  | URLLookupRequest
  | ResourceListRequest
  | ResourceGetRequest
  | ResourceShowRequest
  | DumpRequest
  | DiffRequest;
export type ClientFrame = ClientRequest | Cancel;

export type ItemKind = "catalog_resource" | "url_classification" | "projected_record" | "diff_resource" | "diff_added" | "diff_removed" | "diff_field_change";

export type FieldClassification = "public_project_data" | "operational_metadata" | "tenant_configuration" | "sensitive_identifier" | "free_text" | "secret";

export interface CatalogField {
  readonly name: string;
  readonly json_name?: string;
  readonly classification: FieldClassification;
  readonly allowed_modes: readonly Redaction[];
  readonly fields: readonly CatalogField[];
}

export interface CatalogResource {
  readonly product: Product;
  readonly name: string;
  readonly shape: "list" | "singleton";
  readonly operations: readonly ("list" | "get" | "show")[];
  readonly get_key?: string;
  readonly fields: readonly CatalogField[];
}

export interface URLClassification {
  readonly url: string;
  readonly classifications: readonly string[];
  readonly security_alert_classifications: readonly string[];
  readonly application: string;
}

export type WireValue = JsonValue;
export type WireRecord = JsonObject;

export interface ProjectedRecord {
  readonly product: Product;
  readonly resource: string;
  readonly record: WireRecord;
}

export type DiffIdentity =
  | { readonly mode: "get_key"; readonly field: string }
  | { readonly mode: "singleton" }
  | { readonly mode: "content_hash" };

export interface DiffResource {
  readonly product: Product;
  readonly resource: string;
  readonly identity: DiffIdentity;
  readonly added: SafeInteger;
  readonly removed: SafeInteger;
  readonly changed_fields: SafeInteger;
  readonly note?: string;
}

export type DiffRecordRef =
  | { readonly product: Product; readonly resource: string; readonly key: string; readonly record: WireRecord }
  | { readonly product: Product; readonly resource: string; readonly hash: string; readonly record: WireRecord };

export interface DiffFieldChange {
  readonly product: Product;
  readonly resource: string;
  readonly key: string;
  readonly field: string;
  readonly old: WireValue;
  readonly new: WireValue;
}

export type ItemValue = CatalogResource | URLClassification | ProjectedRecord | DiffResource | DiffRecordRef | DiffFieldChange;

export interface ItemFrame {
  readonly type: "item";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly kind: ItemKind;
  readonly item: ItemValue;
}

export interface ItemBegin {
  readonly type: "item_begin";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly item_id: SafeInteger;
  readonly kind: ItemKind;
  readonly encoding: "json";
  readonly bytes: SafeInteger;
}

export interface ItemChunk {
  readonly type: "item_chunk";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly item_id: SafeInteger;
  readonly index: SafeInteger;
  readonly data: string;
}

export interface ItemEnd {
  readonly type: "item_end";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly item_id: SafeInteger;
  readonly chunks: SafeInteger;
  readonly sha256: string;
}

export interface Progress {
  readonly type: "progress";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly phase: "resource_started";
  readonly current: SafeInteger;
  readonly total: SafeInteger;
  readonly product: Product;
  readonly resource: string;
}

export type DumpFailure =
  | { readonly product: Product; readonly resource: string; readonly phase: "list"; readonly kind: "list_failed" }
  | { readonly product: Product; readonly resource: string; readonly phase: "show"; readonly kind: "show_failed" }
  | { readonly product: Product; readonly resource: string; readonly phase: "project"; readonly kind: "projection_failed" }
  | { readonly product: Product; readonly resource: string; readonly phase: "validate"; readonly kind: "subset_failed" };

export interface Warning {
  readonly type: "warning";
  readonly id: SafeInteger;
  readonly seq: SafeInteger;
  readonly warning: DumpFailure;
}

export interface EngineManifestResult {
  readonly kind: "engine_manifest";
  readonly manifest: EngineManifest;
}
export interface CatalogSummary {
  readonly kind: "catalog_summary";
  readonly resources: SafeInteger;
  readonly stream_items_emitted: SafeInteger;
}
export interface DoctorStatus {
  readonly status: string;
  readonly mode: string;
  readonly profile: string;
  readonly config: string;
  readonly auth_mode: "oneapi" | "zia-legacy";
  readonly redaction: Redaction;
  readonly timeout: string;
  readonly cache: string;
  readonly proxy: string;
  readonly credentials: string;
  readonly live_api: string;
}
export interface DoctorStatusResult { readonly kind: "doctor_status"; readonly status: DoctorStatus; }
export interface AuthStatus { readonly credentials: string; readonly credential_exchange: string; readonly live_api: string; }
export interface AuthStatusResult { readonly kind: "auth_status"; readonly status: AuthStatus; }
export interface ConfigCredentials { readonly client_id_set: boolean; readonly client_secret_set: boolean; readonly client_secret_file_set: boolean; readonly client_secret_scheme?: string; }
export interface ConfigZPA { readonly customer_id_set: boolean; readonly microtenant_id_set: boolean; }
export interface ConfigZIALegacy { readonly username_set: boolean; readonly password_set: boolean; readonly password_file_set: boolean; readonly password_scheme?: string; readonly api_key_set: boolean; readonly api_key_file_set: boolean; readonly api_key_scheme?: string; readonly cloud_set: boolean; }
export interface ConfigProxy { readonly url_set: boolean; readonly from_environment: boolean; }
export interface ConfigDefaults { readonly redaction: Redaction; readonly no_cache: boolean; }
export interface ConfigStatus { readonly source: string; readonly config_file_set: boolean; readonly profile: string; readonly auth_mode: "oneapi" | "zia-legacy"; readonly vanity_domain_set: boolean; readonly cloud?: string; readonly credentials: ConfigCredentials; readonly zpa: ConfigZPA; readonly zia_legacy: ConfigZIALegacy; readonly proxy: ConfigProxy; readonly defaults: ConfigDefaults; }
export interface ConfigStatusResult { readonly kind: "config_status"; readonly status: ConfigStatus; }
export interface URLLookupSummary { readonly kind: "url_lookup_summary"; readonly classifications: SafeInteger; readonly stream_items_emitted: SafeInteger; }
export interface ResourceReadSummary { readonly kind: "resource_read_summary"; readonly records: SafeInteger; readonly stream_items_emitted: SafeInteger; }
export interface DumpSummary { readonly kind: "dump_summary"; readonly records_written: SafeInteger; readonly resources_written: SafeInteger; readonly warning_count: SafeInteger; readonly partial: boolean; readonly redaction: Redaction; readonly failures: readonly DumpFailure[]; readonly stream_items_emitted: 0; }
export interface DumpSideRef { readonly side: "old" | "new"; readonly manifest_schema: string; readonly redaction: Redaction; readonly status: "complete" | "partial"; readonly partial: boolean; }
export interface DiffCounts { readonly resources_compared: SafeInteger; readonly resources_with_drift: SafeInteger; readonly records_added: SafeInteger; readonly records_removed: SafeInteger; readonly records_changed: SafeInteger; }
export interface DiffSummary { readonly kind: "diff_summary"; readonly schema: "zscalerctl.diff.v1"; readonly old: DumpSideRef & { readonly side: "old" }; readonly new: DumpSideRef & { readonly side: "new" }; readonly summary: DiffCounts; readonly has_drift: boolean; readonly stream_items_emitted: SafeInteger; }
export type CompletionResult = EngineManifestResult | CatalogSummary | DoctorStatusResult | AuthStatusResult | ConfigStatusResult | URLLookupSummary | ResourceReadSummary | DumpSummary | DiffSummary;

export interface Completed { readonly type: "completed"; readonly id: SafeInteger; readonly seq: SafeInteger; readonly result: CompletionResult; }
export type FailureKind = "usage" | "unsupported_capability" | "unsupported_operation" | "unknown_resource" | "not_found" | "invalid_resource_id" | "live_access_failed" | "deadline_exceeded" | "invalid_config" | "invalid_proxy_config" | "unsupported_resource" | "response_too_large" | "internal";
export interface MissingCredentialsFailure { readonly kind: "missing_credentials"; readonly missing?: readonly string[]; }
export interface NonCredentialFailure { readonly kind: FailureKind; }
export type OperationFailure = MissingCredentialsFailure | NonCredentialFailure;
export interface Failed { readonly type: "failed"; readonly id: SafeInteger; readonly seq: SafeInteger; readonly error: OperationFailure; }
export interface Canceled { readonly type: "canceled"; readonly id: SafeInteger; readonly seq: SafeInteger; readonly error: { readonly kind: "canceled" }; }
export interface RequestRejected { readonly type: "request_rejected"; readonly id: SafeInteger; readonly reason: "busy"; }
export interface Started { readonly type: "started"; readonly id: SafeInteger; readonly seq: SafeInteger; readonly capability: Capability; readonly operation: Operation; }
export type ServerFrame = Ready | RequestRejected | Started | ItemFrame | ItemBegin | ItemChunk | ItemEnd | Progress | Warning | Completed | Failed | Canceled | BootstrapProtocolError;

export type JsonNumber = WireNumber;
