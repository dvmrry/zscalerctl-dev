export const OVERLAY_Z_INDEX = Object.freeze({
  popover: 100,
  drawer: 1_000,
  dialog: 3_000,
  utility: 3_100,
  toast: 3_200
});

export type OverlayLayer = keyof typeof OVERLAY_Z_INDEX;
export type FloatingWindowPlacement = "top-center" | "center";

export interface FloatingWindowLayoutOptions {
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly preferredWidth: number;
  readonly preferredHeight: number;
  readonly placement?: FloatingWindowPlacement;
  readonly margin?: number;
}

export interface FloatingWindowRect {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly height: number;
}

function positiveInteger(value: number): number {
  if (!Number.isFinite(value)) return 1;
  return Math.max(1, Math.floor(value));
}

function insetFor(viewport: number, requested: number): number {
  const maximum = Math.max(0, Math.floor((viewport - 1) / 2));
  if (!Number.isFinite(requested)) return 0;
  return Math.max(0, Math.min(Math.floor(requested), maximum));
}

export function placeFloatingWindow(options: FloatingWindowLayoutOptions): FloatingWindowRect {
  const viewportWidth = positiveInteger(options.viewportWidth);
  const viewportHeight = positiveInteger(options.viewportHeight);
  const requestedMargin = options.margin ?? 1;
  const horizontalInset = insetFor(viewportWidth, requestedMargin);
  const verticalInset = insetFor(viewportHeight, requestedMargin);
  const availableWidth = Math.max(1, viewportWidth - horizontalInset * 2);
  const availableHeight = Math.max(1, viewportHeight - verticalInset * 2);
  const width = Math.min(positiveInteger(options.preferredWidth), availableWidth);
  const height = Math.min(positiveInteger(options.preferredHeight), availableHeight);
  const horizontalSpace = availableWidth - width;
  const verticalSpace = availableHeight - height;
  const placement = options.placement ?? "top-center";

  return {
    left: horizontalInset + Math.floor(horizontalSpace / 2),
    top: verticalInset + Math.floor(verticalSpace / (placement === "center" ? 2 : 4)),
    width,
    height
  };
}
