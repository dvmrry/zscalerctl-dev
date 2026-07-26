import {safeInlineText, type SafeString} from "./text.ts";

export type ToastTone = "success" | "info" | "warning";
export type ToastColorRole = "success" | "accent" | "warning";

export interface ToastState {
  readonly id: number;
  readonly message: SafeString;
  readonly tone: ToastTone;
}

export type ToastTimerHandle = unknown;

export interface ToastTimerDriver {
  setTimeout(callback: () => void, delayMs: number): ToastTimerHandle;
  clearTimeout(handle: ToastTimerHandle): void;
}

const SYSTEM_TIMERS: ToastTimerDriver = {
  setTimeout: (callback, delayMs) => setTimeout(callback, delayMs),
  clearTimeout: handle => clearTimeout(handle as ReturnType<typeof setTimeout>)
};

export const TOAST_DURATION_MS: Readonly<Record<ToastTone, number>> = Object.freeze({
  success: 2_200,
  info: 3_200,
  warning: 4_500
});

export function toastDurationMs(tone: ToastTone): number {
  return TOAST_DURATION_MS[tone];
}

export function toastMarker(tone: ToastTone): string {
  switch (tone) {
    case "success": return "✓";
    case "info": return "i";
    case "warning": return "!";
  }
}

export function toastColorRole(tone: ToastTone): ToastColorRole {
  switch (tone) {
    case "success": return "success";
    case "info": return "accent";
    case "warning": return "warning";
  }
}

export class LatestToastController {
  readonly #publish: (toast: ToastState | undefined) => void;
  readonly #timers: ToastTimerDriver;
  #generation = 0;
  #activeID: number | undefined;
  #timer: ToastTimerHandle | undefined;

  constructor(
    publish: (toast: ToastState | undefined) => void,
    timers: ToastTimerDriver = SYSTEM_TIMERS
  ) {
    this.#publish = publish;
    this.#timers = timers;
  }

  show(message: string, tone: ToastTone): number {
    if (this.#timer !== undefined) this.#timers.clearTimeout(this.#timer);
    const id = ++this.#generation;
    this.#activeID = id;
    this.#publish({id, message: safeInlineText(message, 500), tone});
    const timer = this.#timers.setTimeout(() => {
      if (this.#activeID !== id || this.#timer !== timer) return;
      this.#activeID = undefined;
      this.#timer = undefined;
      this.#publish(undefined);
    }, toastDurationMs(tone));
    this.#timer = timer;
    return id;
  }

  dismiss(id: number): boolean {
    if (this.#activeID !== id) return false;
    this.#generation += 1;
    this.#activeID = undefined;
    if (this.#timer !== undefined) this.#timers.clearTimeout(this.#timer);
    this.#timer = undefined;
    this.#publish(undefined);
    return true;
  }

  dispose(): void {
    this.#generation += 1;
    this.#activeID = undefined;
    if (this.#timer !== undefined) this.#timers.clearTimeout(this.#timer);
    this.#timer = undefined;
  }
}
