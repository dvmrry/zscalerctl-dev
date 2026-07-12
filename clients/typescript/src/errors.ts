export const ERROR_KINDS = {
  FRAME_TOO_LARGE: "frame_too_large",
  UNTERMINATED_FRAME: "unterminated_frame",
  BARE_CARRIAGE_RETURN: "bare_carriage_return",
  EMPTY_FRAME: "empty_frame",
  INVALID_UTF8: "invalid_utf8",
  INVALID_JSON: "invalid_json",
  DUPLICATE_KEY: "duplicate_key",
  JSON_DEPTH: "json_depth",
  INVALID_FRAME: "invalid_frame",
  WRONG_DIRECTION: "wrong_direction",
} as const;

export type WireErrorKind = (typeof ERROR_KINDS)[keyof typeof ERROR_KINDS];

export class WireError extends Error {
  readonly kind: WireErrorKind;

  constructor(kind: WireErrorKind, message: string = kind) {
    super(message);
    this.name = "WireError";
    this.kind = kind;
  }
}

export function errorKind(error: unknown): string | undefined {
  if (error instanceof WireError) {
    return error.kind;
  }
  if (typeof error === "object" && error !== null && "kind" in error) {
    const kind = (error as { kind?: unknown }).kind;
    return typeof kind === "string" ? kind : undefined;
  }
  return undefined;
}

export function fail(kind: WireErrorKind, message?: string): never {
  throw new WireError(kind, message);
}
