export interface SlashCommandDescriptor {
  readonly command: string;
  readonly usage: string;
  readonly summary: string;
  readonly category?: string;
}

export const SLASH_COMMANDS: readonly SlashCommandDescriptor[] = Object.freeze([
  {command: "/demo", usage: "/demo", summary: "Reload the nested sanitized fixture"},
  {command: "/help", usage: "/help", summary: "Show local experiment commands"},
  {command: "/clear", usage: "/clear", summary: "Clear the conversation transcript"},
  {command: "/find", usage: "/find [text]", summary: "Search keys and rendered values in the structured result"},
  {command: "/theme", usage: "/theme [<name> [auto|dark|light]|list|next|mode [auto|dark|light|toggle]]", summary: "Browse themes or change appearance"},
  {command: "/sort", usage: "/sort <index|name|toggle>", summary: "Order named array items by source index or name"},
  {command: "/sidebar", usage: "/sidebar", summary: "Toggle the context rail"},
  {command: "/inspect", usage: "/inspect", summary: "Toggle the wide JSON inspector"},
  {command: "/cancel", usage: "/cancel", summary: "Cancel the active engine operation"},
  {command: "/quit", usage: "/quit", summary: "Exit and restore the terminal"}
]);

export function suggestSlashCommands(
  draft: string,
  commands: readonly SlashCommandDescriptor[] = SLASH_COMMANDS
): readonly SlashCommandDescriptor[] {
  const normalized = draft.trimStart().toLowerCase();
  if (!normalized.startsWith("/") || normalized.includes(" ")) return [];
  return commands.filter(item => item.command.startsWith(normalized)).slice(0, 12);
}

export type InteractionMode = "base" | "drawer" | "inspector" | "search" | "picker";

export interface InteractionState {
  readonly search: boolean;
  readonly picker?: boolean;
  readonly inspector: boolean;
  readonly drawer: boolean;
}

export function activeInteractionMode(state: InteractionState): InteractionMode {
  if (state.picker === true) return "picker";
  if (state.search) return "search";
  if (state.inspector) return "inspector";
  if (state.drawer) return "drawer";
  return "base";
}

export type InteractionCommand =
  | "app.interrupt"
  | "search.toggle"
  | "search.cancel"
  | "search.commit"
  | "search.inspect"
  | "search.next"
  | "search.previous"
  | "search.page-next"
  | "search.page-previous"
  | "search.first"
  | "search.last"
  | "search.copy-value"
  | "search.copy-path"
  | "picker.cancel"
  | "picker.commit"
  | "picker.next"
  | "picker.previous"
  | "picker.page-next"
  | "picker.page-previous"
  | "picker.first"
  | "picker.last"
  | "sidebar.toggle"
  | "inspector.toggle"
  | "overlay.close"
  | "focus.previous";

export interface KeyStroke {
  readonly name: string;
  readonly ctrl?: boolean;
  readonly meta?: boolean;
  readonly shift?: boolean;
  readonly option?: boolean;
}

function unmodified(key: KeyStroke): boolean {
  return key.ctrl !== true && key.meta !== true && key.option !== true;
}

export function resolveInteractionCommand(mode: InteractionMode, key: KeyStroke): InteractionCommand | undefined {
  const name = key.name.toLowerCase();
  if (key.ctrl === true && name === "c") return "app.interrupt";
  if (key.ctrl === true && name === "f") return "search.toggle";

  if (mode === "picker") {
    if (name === "escape") return "picker.cancel";
    if (unmodified(key) && (name === "return" || name === "kpenter")) return "picker.commit";
    if (unmodified(key) && name === "up") return "picker.previous";
    if (unmodified(key) && name === "down") return "picker.next";
    if (unmodified(key) && name === "pageup") return "picker.page-previous";
    if (unmodified(key) && name === "pagedown") return "picker.page-next";
    if (unmodified(key) && name === "home") return "picker.first";
    if (unmodified(key) && name === "end") return "picker.last";
    return undefined;
  }

  if (mode === "search") {
    if (name === "escape") return "search.cancel";
    if (key.ctrl === true && name === "o") return "search.inspect";
    if (unmodified(key) && (name === "return" || name === "kpenter")) return "search.commit";
    if (unmodified(key) && name === "up") return "search.previous";
    if (unmodified(key) && name === "down") return "search.next";
    if (unmodified(key) && name === "pageup") return "search.page-previous";
    if (unmodified(key) && name === "pagedown") return "search.page-next";
    if (unmodified(key) && name === "home") return "search.first";
    if (unmodified(key) && name === "end") return "search.last";
    // Uppercase action keys remain distinct from ordinary query input.
    if (unmodified(key) && key.shift === true && name === "c") return "search.copy-value";
    if (unmodified(key) && key.shift === true && name === "p") return "search.copy-path";
    return undefined;
  }

  if (key.ctrl === true && name === "b") return "sidebar.toggle";
  if (key.ctrl === true && name === "o") return "inspector.toggle";
  if (name === "escape") return "overlay.close";
  if (unmodified(key) && key.shift === true && name === "tab") return "focus.previous";
  return undefined;
}
