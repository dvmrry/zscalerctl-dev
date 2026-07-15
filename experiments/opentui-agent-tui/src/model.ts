import type {WireValue} from "../../../clients/typescript/src/index.ts";
import {
  SLASH_COMMANDS,
  suggestSlashCommands,
  type SlashCommandDescriptor
} from "./commands.ts";
import type {JsonPath, TreeKind} from "./tree.ts";

export type Tone = "neutral" | "info" | "success" | "warning" | "danger";
export type FocusTarget = "composer" | "tree" | "transcript" | "search" | "picker";

export interface TranscriptMetric {
  readonly label: string;
  readonly value: string;
  readonly tone?: Tone;
}

export interface TranscriptFacetValue {
  readonly label: string;
  readonly count: number;
}

export interface TranscriptFacet {
  readonly label: string;
  readonly values: readonly TranscriptFacetValue[];
}

export interface TranscriptEvidence {
  readonly path: JsonPath;
  readonly label: string;
  readonly detail: string;
  readonly kind: TreeKind;
  readonly preview: string;
}

export type TranscriptActionID = "inspect" | "find";

export interface TranscriptAction {
  readonly id: TranscriptActionID;
  readonly label: string;
  readonly hint?: string;
}

export type TranscriptBlock =
  | {readonly id: string; readonly kind: "text"; readonly lines: readonly string[]}
  | {readonly id: string; readonly kind: "metrics"; readonly items: readonly TranscriptMetric[]}
  | {readonly id: string; readonly kind: "facets"; readonly items: readonly TranscriptFacet[]}
  | {readonly id: string; readonly kind: "evidence"; readonly items: readonly TranscriptEvidence[]}
  | {readonly id: string; readonly kind: "actions"; readonly items: readonly TranscriptAction[]};

export interface TranscriptEntry {
  readonly id: number;
  readonly role: "user" | "assistant" | "system";
  readonly title?: string;
  readonly blocks: readonly TranscriptBlock[];
  readonly tone?: Tone;
  readonly resultId?: number;
}

export interface PinnedEvidence extends TranscriptEvidence {
  readonly id: number;
  readonly ref: EvidenceReference;
}

export interface EvidenceReference {
  readonly resultId: number;
  readonly path: JsonPath;
}

export interface ResultSnapshot {
  readonly id: number;
  readonly data: WireValue;
  readonly context?: ContextState;
}

export interface OperationState {
  readonly status: "idle" | "running" | "complete" | "error";
  readonly label: string;
  /** Work units fully completed; the in-flight unit is never included. */
  readonly completed?: number;
  readonly total?: number;
}

export interface ContextState {
  readonly connection: "fixture" | "connecting" | "connected" | "error";
  readonly transport: string;
  readonly authority: string;
  readonly scope: string;
  readonly records: number;
  readonly effects: string;
  readonly operation: OperationState;
}

export type CommandDescriptor = SlashCommandDescriptor;
export const COMMANDS = SLASH_COMMANDS;

export function commandSuggestions(draft: string): readonly CommandDescriptor[] {
  return suggestSlashCommands(draft);
}

export function suggestionsFor(
  draft: string,
  commands: readonly CommandDescriptor[]
): readonly CommandDescriptor[] {
  return suggestSlashCommands(draft, commands);
}

export function toneColor(tone: Tone | undefined, colors: {
  readonly text: string;
  readonly accent: string;
  readonly success: string;
  readonly warning: string;
  readonly danger: string;
}): string {
  switch (tone) {
    case "info": return colors.accent;
    case "success": return colors.success;
    case "warning": return colors.warning;
    case "danger": return colors.danger;
    default: return colors.text;
  }
}
