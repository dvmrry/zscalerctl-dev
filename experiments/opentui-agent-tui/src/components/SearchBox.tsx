import {useMemo} from "react";

import type {Palette} from "../theme.ts";
import {safeInlineText, type SafeString} from "../text.ts";
import type {TreeSearchMatch} from "../tree.ts";
import {
  PickerWindow,
  type PickerAction,
  type PickerInputMethod,
  type PickerItem
} from "./PickerWindow.tsx";

export interface SearchBoxProps {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly preferredWidth: number;
  readonly focused: boolean;
  readonly query: string;
  readonly selectedId?: string;
  readonly matches: readonly TreeSearchMatch[];
  readonly truncated: boolean;
  readonly inputMethod: PickerInputMethod;
  readonly onInput: (value: string) => void;
  readonly onFocus: () => void;
  readonly onInputMethodChange: (method: PickerInputMethod) => void;
  readonly onMove: (id: string) => void;
  readonly onCommit: (id: string) => void;
  readonly onInspect: (id: string) => void;
  readonly onCopyValue: (id: string) => void;
  readonly onCopyPath: (id: string) => void;
  readonly onCancel: () => void;
}

function reasonLabel(match: TreeSearchMatch): SafeString {
  switch (match.reason) {
    case "identity": return safeInlineText("name");
    case "key": return safeInlineText("field");
    case "key-value": return safeInlineText("field + value");
    case "value": return safeInlineText("value");
  }
}

export function SearchBox(props: SearchBoxProps) {
  const items = useMemo<readonly PickerItem<TreeSearchMatch>[]>(() => props.matches.map(match => ({
    id: match.id,
    value: match,
    title: match.context.length === 0 ? safeInlineText("Value") : match.context,
    description: match.detail,
    category: match.title,
    categoryId: match.groupId,
    badge: reasonLabel(match)
  })), [props.matches]);
  const actions: readonly PickerAction<TreeSearchMatch>[] = [
    {
      id: "reveal",
      shortcut: safeInlineText("Enter"),
      compactShortcut: safeInlineText("↵"),
      label: safeInlineText("reveal"),
      compactLabel: safeInlineText("open"),
      disabled: item => item === undefined,
      onTrigger: item => { if (item !== undefined) props.onCommit(item.id); }
    },
    {
      id: "inspect",
      shortcut: safeInlineText("Ctrl+O"),
      compactShortcut: safeInlineText("^O"),
      label: safeInlineText("inspect"),
      compactLabel: safeInlineText("view"),
      disabled: item => item === undefined,
      onTrigger: item => { if (item !== undefined) props.onInspect(item.id); }
    },
    {
      id: "copy-value",
      shortcut: safeInlineText("C"),
      label: safeInlineText("copy value"),
      compactLabel: safeInlineText("value"),
      disabled: item => item?.value.copyText === undefined,
      onTrigger: item => { if (item !== undefined) props.onCopyValue(item.id); }
    },
    {
      id: "copy-path",
      shortcut: safeInlineText("P"),
      label: safeInlineText("copy path"),
      compactLabel: safeInlineText("path"),
      disabled: item => item === undefined,
      onTrigger: item => { if (item !== undefined) props.onCopyPath(item.id); }
    }
  ];

  return (
    <PickerWindow
      colors={props.colors}
      viewportWidth={props.viewportWidth}
      viewportHeight={props.viewportHeight}
      preferredWidth={props.preferredWidth}
      title={safeInlineText("Find in structured data")}
      query={props.query}
      placeholder={safeInlineText("key, name, or rendered value")}
      focused={props.focused}
      items={items}
      selectedId={props.selectedId}
      truncated={props.truncated}
      instruction={safeInlineText("Search sanitized names, fields, and exact scalar values.")}
      emptyMessage={safeInlineText("No matching structured values.")}
      inputMethod={props.inputMethod}
      actions={actions}
      onInput={props.onInput}
      onFocus={props.onFocus}
      onInputMethodChange={props.onInputMethodChange}
      onMove={props.onMove}
      onSelect={props.onCommit}
      onCancel={props.onCancel}
    />
  );
}
