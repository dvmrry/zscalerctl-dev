export type ToastTone = "success" | "info" | "warning";
export type ToastColorRole = "success" | "accent" | "warning";

export interface ToastState {
  readonly id: number;
  readonly message: string;
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
  #timer: ToastTimerHandle | undefined;

  constructor(
    publish: (toast: ToastState | undefined) => void,
    timers: ToastTimerDriver = SYSTEM_TIMERS
  ) {
    this.#publish = publish;
    this.#timers = timers;
  }

  show(message: string, tone: ToastTone): void {
    if (this.#timer !== undefined) this.#timers.clearTimeout(this.#timer);
    const id = ++this.#generation;
    this.#publish({id, message, tone});
    const timer = this.#timers.setTimeout(() => {
      if (this.#generation !== id || this.#timer !== timer) return;
      this.#timer = undefined;
      this.#publish(undefined);
    }, toastDurationMs(tone));
    this.#timer = timer;
  }

  dispose(): void {
    this.#generation += 1;
    if (this.#timer !== undefined) this.#timers.clearTimeout(this.#timer);
    this.#timer = undefined;
  }
}
