export const MOTION_MODES = ["full", "reduced", "off"] as const;

export type MotionMode = (typeof MOTION_MODES)[number];

export const DEFAULT_MOTION_MODE: MotionMode = "full";
export const FULL_MOTION_INTERVAL_MS = 120;
export const REDUCED_MOTION_INTERVAL_MS = 420;
export const OPERATION_SCENE_DELAY_MS = 320;
export const WELCOME_MOTION_DURATION_MS = 1_800;

export type MotionTimerHandle = unknown;

export interface MotionTimerDriver {
  setInterval(callback: () => void, delayMs: number): MotionTimerHandle;
  clearInterval(handle: MotionTimerHandle): void;
  setTimeout(callback: () => void, delayMs: number): MotionTimerHandle;
  clearTimeout(handle: MotionTimerHandle): void;
}

export const SYSTEM_MOTION_TIMERS: MotionTimerDriver = {
  setInterval: (callback, delayMs) => setInterval(callback, delayMs),
  clearInterval: handle => clearInterval(handle as ReturnType<typeof setInterval>),
  setTimeout: (callback, delayMs) => setTimeout(callback, delayMs),
  clearTimeout: handle => clearTimeout(handle as ReturnType<typeof setTimeout>)
};

export function isMotionMode(value: string): value is MotionMode {
  return (MOTION_MODES as readonly string[]).includes(value);
}

export function motionInterval(mode: MotionMode): number | undefined {
  switch (mode) {
    case "full": return FULL_MOTION_INTERVAL_MS;
    case "reduced": return REDUCED_MOTION_INTERVAL_MS;
    case "off": return undefined;
  }
}

export function motionDescription(mode: MotionMode): string {
  switch (mode) {
    case "full": return "decorative scenes and normal-speed liveness";
    case "reduced": return "slower liveness with decorative sweeps held still";
    case "off": return "static artwork and activity indicators";
  }
}
