import type {WireValue} from "../../../clients/typescript/src/index.ts";

import {DEMO_DATA} from "./fixture.ts";
import type {CommandDescriptor, ContextState, Tone} from "./model.ts";
import {safeInlineText} from "./text.ts";

export interface WorkspaceAnnouncement {
  readonly title: string;
  readonly body: readonly string[];
  readonly tone: Tone;
}

export interface WorkspaceSnapshot {
  readonly data: WireValue;
  readonly context: ContextState;
  readonly announcement: WorkspaceAnnouncement;
}

export interface WorkspaceProgressEvent {
  readonly kind: "progress";
  readonly current: number;
  readonly total: number;
  readonly message: string;
}

export interface WorkspaceExecutionContext {
  readonly signal: AbortSignal;
  readonly emit: (event: WorkspaceProgressEvent) => void;
}

export interface WorkspacePickerItem {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly searchText?: string;
  readonly category: string;
  readonly badge?: string;
  readonly command: string;
}

export interface WorkspacePicker {
  readonly title: string;
  readonly placeholder: string;
  readonly instruction: string;
  readonly emptyMessage: string;
  readonly items: readonly WorkspacePickerItem[];
  readonly initialQuery?: string;
}

export interface WorkspaceResult {
  readonly announcement: WorkspaceAnnouncement;
  readonly data?: WireValue;
  readonly context?: ContextState;
  readonly picker?: WorkspacePicker;
}

export interface WorkspaceAdapter {
  readonly id: string;
  readonly initial: WorkspaceSnapshot;
  readonly commands?: readonly CommandDescriptor[];
  connect?(context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  execute?(input: string, context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  reload?(context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  close(): Promise<void>;
}

export class WorkspaceCommandError extends Error {
  readonly title: string;
  readonly tone: Tone;
  readonly details: readonly string[];
  readonly canceled: boolean;

  constructor(options: {
    readonly title: string;
    readonly message: string;
    readonly tone?: Tone;
    readonly details?: readonly string[];
    readonly canceled?: boolean;
  }) {
    super(options.message);
    this.name = "WorkspaceCommandError";
    this.title = options.title;
    this.tone = options.tone ?? "danger";
    this.details = options.details ?? [];
    this.canceled = options.canceled ?? false;
  }
}

export interface FilteredWorkspacePicker {
  readonly items: readonly WorkspacePickerItem[];
  readonly truncated: boolean;
}

export function normalizeWorkspacePicker(picker: WorkspacePicker): WorkspacePicker {
  return {
    title: safeInlineText(picker.title, 180),
    placeholder: safeInlineText(picker.placeholder, 180),
    instruction: safeInlineText(picker.instruction, 500),
    emptyMessage: safeInlineText(picker.emptyMessage, 500),
    items: picker.items.map(item => ({
      ...item,
      title: safeInlineText(item.title, 240),
      description: safeInlineText(item.description, 500),
      category: safeInlineText(item.category, 120),
      ...(item.badge === undefined ? {} : {badge: safeInlineText(item.badge, 80)}),
      ...(item.searchText === undefined ? {} : {searchText: safeInlineText(item.searchText, 4_096)})
    })),
    ...(picker.initialQuery === undefined ? {} : {initialQuery: safeInlineText(picker.initialQuery, 120)})
  };
}

export function filterWorkspacePicker(
  items: readonly WorkspacePickerItem[],
  query: string,
  limit = 80
): FilteredWorkspacePicker {
  const terms = query.trim().toLowerCase().split(/\s+/u).filter(Boolean);
  const matched = terms.length === 0
    ? items
    : items.filter(item => {
        const text = `${item.category} ${item.title} ${item.description} ${item.searchText ?? ""} ${item.badge ?? ""}`.toLowerCase();
        return terms.every(term => text.includes(term));
      });
  return {items: matched.slice(0, limit), truncated: matched.length > limit};
}

const FIXTURE_CONTEXT: ContextState = {
  connection: "fixture",
  transport: "in-memory fixture",
  authority: "tenant read-only",
  scope: "zia/locations",
  records: 2,
  effects: "none",
  operation: {status: "idle", label: "ready"}
};

const INITIAL_FIXTURE: WorkspaceSnapshot = {
  data: DEMO_DATA,
  context: FIXTURE_CONTEXT,
  announcement: {
    title: "Structured fixture ready",
    body: [
      "A nested, sanitized location result is loaded without credentials.",
      "The large numeric identifier remains an exact wire lexeme rather than a JavaScript float."
    ],
    tone: "success"
  }
};

function delay(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new WorkspaceCommandError({title: "Operation canceled", message: "The local operation was canceled.", tone: "warning", canceled: true}));
      return;
    }
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", canceled);
      resolve();
    }, milliseconds);
    const canceled = () => {
      clearTimeout(timer);
      reject(new WorkspaceCommandError({title: "Operation canceled", message: "The local operation was canceled.", tone: "warning", canceled: true}));
    };
    signal.addEventListener("abort", canceled, {once: true});
  });
}

export const FIXTURE_WORKSPACE_ADAPTER: WorkspaceAdapter = Object.freeze({
  id: "fixture",
  initial: INITIAL_FIXTURE,
  async reload(context: WorkspaceExecutionContext): Promise<WorkspaceResult> {
    await delay(480, context.signal);
    return {
      data: DEMO_DATA,
      context: {...FIXTURE_CONTEXT, operation: {status: "complete", label: "2 records projected"}},
      announcement: {
        title: "Fixture reloaded",
        body: ["The context tree has been reset to its initial expansion state."],
        tone: "success"
      }
    };
  },
  async close(): Promise<void> {
    // The fixture owns no external resources.
  }
});
