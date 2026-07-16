import type {Filter, Product, ResourceSelector} from "../../../../clients/typescript/src/index.ts";

import type {SlashCommandDescriptor} from "../commands.ts";
import {containsUnsafeFormatCharacter} from "../text.ts";

const PRODUCTS = new Set<Product>(["zia", "zpa", "ztw", "zcc", "zidentity"]);
const MAXIMUM_COMMAND_CHARACTERS = 8_192;
const MAXIMUM_TOKENS = 256;

export class CommandSyntaxError extends Error {
  readonly usage?: string;

  constructor(message: string, usage?: string) {
    super(message);
    this.name = "CommandSyntaxError";
    this.usage = usage;
  }
}

export type ParsedCommand =
  | {readonly kind: "manifest"}
  | {readonly kind: "catalog"; readonly product?: Product; readonly query: string}
  | {readonly kind: "doctor"}
  | {readonly kind: "auth"}
  | {readonly kind: "config"}
  | {readonly kind: "lookup"; readonly urls: readonly string[]}
  | {
      readonly kind: "list";
      readonly product: Product;
      readonly resource: string;
      readonly fields: readonly string[];
      readonly filters: readonly Filter[];
      readonly search: string;
    }
  | {
      readonly kind: "get";
      readonly product: Product;
      readonly resource: string;
      readonly recordID: string;
      readonly fields: readonly string[];
    }
  | {
      readonly kind: "show";
      readonly product: Product;
      readonly resource: string;
      readonly fields: readonly string[];
    }
  | {
      readonly kind: "diff";
      readonly oldDirectory: string;
      readonly newDirectory: string;
      readonly products: readonly Product[];
      readonly resources: readonly ResourceSelector[];
      readonly ignoreOperational: boolean;
      readonly allowPartial: boolean;
    };

export const ZSCALERCTL_COMMANDS: readonly SlashCommandDescriptor[] = Object.freeze([
  {command: "/manifest", usage: "/manifest", summary: "Show negotiated engine capabilities", category: "discovery"},
  {command: "/catalog", usage: "/catalog [product] [text]", summary: "Browse known projected resources", category: "discovery"},
  {command: "/doctor", usage: "/doctor", summary: "Inspect configuration and credential readiness", category: "status"},
  {command: "/auth", usage: "/auth", summary: "Inspect authentication readiness", category: "status"},
  {command: "/config", usage: "/config", summary: "Inspect safe configuration metadata", category: "status"},
  {command: "/lookup", usage: "/lookup <url> [url…]", summary: "Classify one or more URLs through ZIA", category: "read"},
  {command: "/list", usage: "/list <product> <resource> [options]", summary: "Read a projected resource collection", category: "read"},
  {command: "/get", usage: "/get <product> <resource> <id> [--fields a,b]", summary: "Read one projected resource record", category: "read"},
  {command: "/show", usage: "/show <product> <resource> [--fields a,b]", summary: "Read a projected singleton resource", category: "read"},
  {command: "/diff", usage: "/diff <old-dir> <new-dir> [options]", summary: "Compare two existing sanitized dumps", category: "local"}
]);

export function tokenizeCommand(input: string): readonly string[] {
  if (typeof input !== "string" || input.length === 0 || input.length > MAXIMUM_COMMAND_CHARACTERS
      || /[\u0000-\u001f\u007f-\u009f]/u.test(input) || containsUnsafeFormatCharacter(input)) {
    throw new CommandSyntaxError("Command input contains an invalid control character or exceeds the local limit.");
  }
  const tokens: string[] = [];
  let token = "";
  let quote: "'" | "\"" | undefined;
  let escaped = false;
  let started = false;
  for (const character of input) {
    if (escaped) {
      token += character;
      escaped = false;
      started = true;
      continue;
    }
    if (character === "\\") {
      escaped = true;
      started = true;
      continue;
    }
    if (quote !== undefined) {
      if (character === quote) quote = undefined;
      else token += character;
      started = true;
      continue;
    }
    if (character === "'" || character === "\"") {
      quote = character;
      started = true;
      continue;
    }
    if (/\s/u.test(character)) {
      if (started) {
        tokens.push(token);
        token = "";
        started = false;
      }
      continue;
    }
    token += character;
    started = true;
  }
  if (escaped) throw new CommandSyntaxError("A trailing backslash is not valid command syntax.");
  if (quote !== undefined) throw new CommandSyntaxError("A quoted command argument was not terminated.");
  if (started) tokens.push(token);
  if (tokens.length > MAXIMUM_TOKENS) throw new CommandSyntaxError("Command input has too many arguments.");
  return tokens;
}

function product(value: string, usage: string): Product {
  const normalized = value.toLowerCase() as Product;
  if (!PRODUCTS.has(normalized)) throw new CommandSyntaxError(`Unknown product ${JSON.stringify(value)}.`, usage);
  return normalized;
}

function required(tokens: readonly string[], index: number, name: string, usage: string): string {
  const value = tokens[index];
  if (value === undefined || value.length === 0 || value.startsWith("--")) {
    throw new CommandSyntaxError(`${name} is required.`, usage);
  }
  return value;
}

function optionValue(
  tokens: readonly string[],
  index: number,
  name: string,
  usage: string
): {readonly value: string; readonly next: number} | undefined {
  const token = tokens[index];
  if (token === undefined) return undefined;
  if (token === name) {
    const value = tokens[index + 1];
    if (value === undefined || value.startsWith("--")) throw new CommandSyntaxError(`${name} requires a value.`, usage);
    return {value, next: index + 2};
  }
  const prefix = `${name}=`;
  if (token.startsWith(prefix)) {
    const value = token.slice(prefix.length);
    if (value.length === 0) throw new CommandSyntaxError(`${name} requires a value.`, usage);
    return {value, next: index + 1};
  }
  return undefined;
}

function fields(value: string, usage: string): readonly string[] {
  const output = value.split(",").map(field => field.trim());
  if (output.some(field => field.length === 0)) throw new CommandSyntaxError("--fields contains an empty field name.", usage);
  return [...new Set(output)];
}

function filter(value: string, usage: string): Filter {
  const contains = value.indexOf("~");
  const exact = value.indexOf("=");
  const index = contains < 0 ? exact : exact < 0 ? contains : Math.min(contains, exact);
  const field = index < 0 ? "" : value.slice(0, index).trim();
  if (index < 0 || field.length === 0) throw new CommandSyntaxError("--filter must be field=value or field~value.", usage);
  return {field, operator: value[index] === "~" ? "contains" : "exact", value: value.slice(index + 1)};
}

function parseReadOptions(
  tokens: readonly string[],
  start: number,
  usage: string,
  allowFilter: boolean
): {readonly fields: readonly string[]; readonly filters: readonly Filter[]; readonly search: string} {
  let selectedFields: readonly string[] = [];
  const filters: Filter[] = [];
  let search = "";
  let index = start;
  while (index < tokens.length) {
    const fieldOption = optionValue(tokens, index, "--fields", usage);
    if (fieldOption !== undefined) {
      if (selectedFields.length > 0) throw new CommandSyntaxError("--fields may be supplied only once.", usage);
      selectedFields = fields(fieldOption.value, usage);
      index = fieldOption.next;
      continue;
    }
    const filterOption = optionValue(tokens, index, "--filter", usage);
    if (filterOption !== undefined && allowFilter) {
      filters.push(filter(filterOption.value, usage));
      index = filterOption.next;
      continue;
    }
    const searchOption = optionValue(tokens, index, "--search", usage);
    if (searchOption !== undefined && allowFilter) {
      if (search.length > 0) throw new CommandSyntaxError("--search may be supplied only once.", usage);
      search = searchOption.value;
      index = searchOption.next;
      continue;
    }
    throw new CommandSyntaxError(`Unknown option ${JSON.stringify(tokens[index])}.`, usage);
  }
  return {fields: selectedFields, filters, search};
}

function selector(value: string, usage: string): ResourceSelector {
  const separator = value.indexOf("/");
  if (separator <= 0 || separator === value.length - 1 || value.indexOf("/", separator + 1) >= 0) {
    throw new CommandSyntaxError("--resource must be product/resource.", usage);
  }
  return {product: product(value.slice(0, separator), usage), resource: value.slice(separator + 1)};
}

function parseDiff(tokens: readonly string[]): ParsedCommand {
  const usage = "/diff <old-dir> <new-dir> [--product p] [--resource p/name] [--ignore-operational] [--allow-partial]";
  const oldDirectory = required(tokens, 1, "old dump directory", usage);
  const newDirectory = required(tokens, 2, "new dump directory", usage);
  const products: Product[] = [];
  const resources: ResourceSelector[] = [];
  let ignoreOperational = false;
  let allowPartial = false;
  let index = 3;
  while (index < tokens.length) {
    const productOption = optionValue(tokens, index, "--product", usage);
    if (productOption !== undefined) {
      products.push(product(productOption.value, usage));
      index = productOption.next;
      continue;
    }
    const resourceOption = optionValue(tokens, index, "--resource", usage);
    if (resourceOption !== undefined) {
      const selected = selector(resourceOption.value, usage);
      if (!resources.some(resource => resource.product === selected.product && resource.resource === selected.resource)) resources.push(selected);
      index = resourceOption.next;
      continue;
    }
    if (tokens[index] === "--ignore-operational") {
      ignoreOperational = true;
      index += 1;
      continue;
    }
    if (tokens[index] === "--allow-partial") {
      allowPartial = true;
      index += 1;
      continue;
    }
    throw new CommandSyntaxError(`Unknown option ${JSON.stringify(tokens[index])}.`, usage);
  }
  return {
    kind: "diff",
    oldDirectory,
    newDirectory,
    products: [...new Set(products)],
    resources,
    ignoreOperational,
    allowPartial
  };
}

export function parseCommand(input: string): ParsedCommand {
  const tokens = tokenizeCommand(input);
  const command = tokens[0];
  if (command === undefined || !command.startsWith("/")) {
    throw new CommandSyntaxError("Commands begin with /. Use /help to discover the deterministic interface.");
  }
  switch (command.slice(1).toLowerCase()) {
    case "manifest":
      if (tokens.length !== 1) throw new CommandSyntaxError("/manifest takes no arguments.", "/manifest");
      return {kind: "manifest"};
    case "catalog": {
      let selectedProduct: Product | undefined;
      let queryStart = 1;
      if (tokens[1] !== undefined && PRODUCTS.has(tokens[1].toLowerCase() as Product)) {
        selectedProduct = product(tokens[1], "/catalog [product] [text]");
        queryStart = 2;
      }
      const query = tokens.slice(queryStart).join(" ").toLowerCase();
      return selectedProduct === undefined ? {kind: "catalog", query} : {kind: "catalog", product: selectedProduct, query};
    }
    case "doctor":
      if (tokens.length !== 1) throw new CommandSyntaxError("/doctor takes no arguments.", "/doctor");
      return {kind: "doctor"};
    case "auth":
      if (tokens.length !== 1) throw new CommandSyntaxError("/auth takes no arguments.", "/auth");
      return {kind: "auth"};
    case "config":
      if (tokens.length !== 1) throw new CommandSyntaxError("/config takes no arguments.", "/config");
      return {kind: "config"};
    case "lookup": {
      const urls = tokens.slice(1);
      if (urls.length === 0) throw new CommandSyntaxError("At least one URL is required.", "/lookup <url> [url…]");
      return {kind: "lookup", urls};
    }
    case "list": {
      const usage = "/list <product> <resource> [--fields a,b] [--filter field=value] [--search text]";
      const selectedProduct = product(required(tokens, 1, "product", usage), usage);
      const resource = required(tokens, 2, "resource", usage);
      const options = parseReadOptions(tokens, 3, usage, true);
      return {kind: "list", product: selectedProduct, resource, ...options};
    }
    case "get": {
      const usage = "/get <product> <resource> <id> [--fields a,b]";
      const selectedProduct = product(required(tokens, 1, "product", usage), usage);
      const resource = required(tokens, 2, "resource", usage);
      const recordID = required(tokens, 3, "record ID", usage);
      const options = parseReadOptions(tokens, 4, usage, false);
      return {kind: "get", product: selectedProduct, resource, recordID, fields: options.fields};
    }
    case "show": {
      const usage = "/show <product> <resource> [--fields a,b]";
      const selectedProduct = product(required(tokens, 1, "product", usage), usage);
      const resource = required(tokens, 2, "resource", usage);
      const options = parseReadOptions(tokens, 3, usage, false);
      return {kind: "show", product: selectedProduct, resource, fields: options.fields};
    }
    case "diff": return parseDiff(tokens);
    default: throw new CommandSyntaxError(`Unknown command ${JSON.stringify(command)}. Use /help to list supported commands.`);
  }
}

export function describeCommand(input: string): string {
  const command = parseCommand(input);
  switch (command.kind) {
    case "manifest": return "Inspect engine capabilities";
    case "catalog": return "Browse the resource catalog";
    case "doctor": return "Inspect operational readiness";
    case "auth": return "Inspect authentication readiness";
    case "config": return "Inspect configuration metadata";
    case "lookup": return `Classify ${command.urls.length} URL${command.urls.length === 1 ? "" : "s"}`;
    case "list": return `List ${command.product}/${command.resource}`;
    case "get": return `Get ${command.product}/${command.resource}`;
    case "show": return `Show ${command.product}/${command.resource}`;
    case "diff": return "Compare sanitized dumps";
  }
}
