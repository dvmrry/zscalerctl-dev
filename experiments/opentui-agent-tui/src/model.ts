import type {WireValue} from "../../../clients/typescript/src/index.ts";
import {
  SLASH_COMMANDS,
  suggestSlashCommands,
  type SlashCommandDescriptor
} from "./commands.ts";

export type Tone = "neutral" | "info" | "success" | "warning" | "danger";
export type FocusTarget = "composer" | "tree" | "transcript" | "search" | "picker";

export interface TranscriptEntry {
  readonly id: number;
  readonly role: "user" | "assistant" | "system";
  readonly title?: string;
  readonly body: readonly string[];
  readonly tone?: Tone;
  readonly data?: WireValue;
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
