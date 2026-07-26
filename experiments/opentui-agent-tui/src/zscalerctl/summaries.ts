import type {
  AuthStatus,
  CatalogResource,
  ConfigStatus,
  DiffSummary,
  DoctorStatus,
  EngineManifest,
  Operation
} from "../../../../clients/typescript/src/index.ts";

import type {Tone} from "../model.ts";
import type {WorkspaceFacet, WorkspaceMetric, WorkspaceSummary} from "../workspace.ts";

const PRODUCT_ORDER: readonly CatalogResource["product"][] = ["zia", "zpa", "zcc", "ztw", "zidentity"];
const PRODUCT_LABELS: Readonly<Record<CatalogResource["product"], string>> = {
  zia: "ZIA",
  zpa: "ZPA",
  zcc: "ZCC",
  ztw: "ZTW",
  zidentity: "ZIDENTITY"
};
const SHAPE_ORDER: readonly CatalogResource["shape"][] = ["list", "singleton"];
const RESOURCE_OPERATION_ORDER: readonly CatalogResource["operations"][number][] = ["list", "get", "show"];
const MANIFEST_OPERATION_ORDER: readonly Operation[] = [
  "manifest",
  "list",
  "get",
  "show",
  "doctor",
  "auth_status",
  "config_status",
  "lookup",
  "dump",
  "diff"
];
const EFFECT_ORDER: readonly EngineManifest["capabilities"][number]["effects"][number]["kind"][] = [
  "local_filesystem_read",
  "local_filesystem_write",
  "local_filesystem_delete",
  "network_access",
  "process_execution"
];

function countValues<T extends string>(
  values: readonly T[],
  order: readonly T[],
  label: (value: T) => string
): WorkspaceFacet["values"] {
  const counts = new Map<T, number>();
  for (const value of values) counts.set(value, (counts.get(value) ?? 0) + 1);
  return order.flatMap(value => {
    const count = counts.get(value) ?? 0;
    return count === 0 ? [] : [{label: label(value), count}];
  });
}

function metric(label: string, value: string | number, tone?: Tone): WorkspaceMetric {
  return {label, value: String(value), ...(tone === undefined ? {} : {tone})};
}

function words(value: string): string {
  return value.split("_").map(word => `${word.slice(0, 1).toUpperCase()}${word.slice(1)}`).join(" ");
}

export function summarizeCatalog(
  resources: readonly CatalogResource[],
  totalResources: number,
  capabilityCount?: number
): WorkspaceSummary {
  const metrics: WorkspaceMetric[] = [];
  if (resources.length !== totalResources) metrics.push(metric("Available", totalResources));
  if (capabilityCount !== undefined) metrics.push(metric("Capabilities", capabilityCount));
  const facets: WorkspaceFacet[] = [
    {
      label: "Products",
      values: countValues(resources.map(resource => resource.product), PRODUCT_ORDER, product => PRODUCT_LABELS[product])
    },
    {
      label: "Shapes",
      values: countValues(resources.map(resource => resource.shape), SHAPE_ORDER, words)
    },
    {
      label: "Operations",
      values: countValues(resources.flatMap(resource => resource.operations), RESOURCE_OPERATION_ORDER, words)
    }
  ].filter(facet => facet.values.length > 0);
  return {metrics, facets};
}

export function summarizeManifest(manifest: EngineManifest): WorkspaceSummary {
  const operations = manifest.capabilities.flatMap(capability => capability.operations);
  const effects = manifest.capabilities.flatMap(capability => capability.effects.map(effect => effect.kind));
  return {
    metrics: [
      metric("Version", manifest.version),
      metric("Tenant read only", manifest.tenant_read_only ? "yes" : "no", manifest.tenant_read_only ? "success" : "danger")
    ],
    facets: [
      {
        label: "Operations",
        values: countValues(operations, MANIFEST_OPERATION_ORDER, words)
      },
      {
        label: "Declared effects",
        values: countValues(effects, EFFECT_ORDER, words)
      }
    ].filter(facet => facet.values.length > 0)
  };
}

export function summarizeRead(options: {
  readonly operation: "list" | "get" | "show";
  readonly fields: readonly string[];
  readonly filterCount?: number;
  readonly hasSearch?: boolean;
}): WorkspaceSummary {
  const metrics: WorkspaceMetric[] = [
    metric("Operation", options.operation),
    metric("Fields", options.fields.length === 0 ? "default" : options.fields.length)
  ];
  const filterCount = options.filterCount ?? 0;
  if (filterCount > 0) metrics.push(metric("Filters", filterCount));
  if (options.hasSearch === true) metrics.push(metric("Search", "applied"));
  return {metrics};
}

export function summarizeDoctor(status: DoctorStatus): WorkspaceSummary {
  return {
    metrics: [
      metric("Status", status.status, status.status.toLowerCase() === "ok" ? "success" : "warning"),
      metric("Mode", status.mode),
      metric("Auth", status.auth_mode),
      metric("Redaction", status.redaction)
    ],
    notes: [
      `Credentials: ${status.credentials} · Live API: ${status.live_api}`,
      `Profile: ${status.profile} · Config: ${status.config} · Proxy: ${status.proxy}`
    ]
  };
}

export function summarizeAuth(status: AuthStatus): WorkspaceSummary {
  return {
    metrics: [
      metric("Credentials", status.credentials, status.credentials === "configured" ? "success" : "warning"),
      metric("Exchange", status.credential_exchange),
      metric("Live API", status.live_api)
    ]
  };
}

export function summarizeConfig(status: ConfigStatus): WorkspaceSummary {
  const proxy = status.proxy.url_set ? "configured" : status.proxy.from_environment ? "environment" : "not configured";
  return {
    metrics: [
      metric("Source", status.source),
      metric("Profile", status.profile),
      metric("Auth", status.auth_mode),
      metric("Redaction", status.defaults.redaction)
    ],
    notes: [
      `Config file: ${status.config_file_set ? "set" : "not set"} · Vanity domain: ${status.vanity_domain_set ? "set" : "not set"} · Proxy: ${proxy}`
    ]
  };
}

export function summarizeLookup(requestedURLs: number): WorkspaceSummary {
  return {metrics: [metric("URLs requested", requestedURLs)]};
}

export function summarizeDiff(result: DiffSummary): WorkspaceSummary {
  const summary = result.summary;
  const partial = result.old.partial || result.new.partial;
  return {
    metrics: [
      metric("Resources", summary.resources_compared),
      metric("With drift", summary.resources_with_drift, summary.resources_with_drift > 0 ? "warning" : "success")
    ],
    facets: [{
      label: "Record changes",
      values: [
        {label: "Added", count: summary.records_added},
        {label: "Removed", count: summary.records_removed},
        {label: "Changed", count: summary.records_changed}
      ]
    }],
    notes: [
      `Old: ${result.old.status} · ${result.old.redaction} redaction; New: ${result.new.status} · ${result.new.redaction} redaction`,
      ...(partial ? ["Partial comparison: uncollected resources were not compared."] : [])
    ]
  };
}
