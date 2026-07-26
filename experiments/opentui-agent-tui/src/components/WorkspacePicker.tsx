import type {Palette} from "../theme.ts";
import {safeInlineText} from "../text.ts";
import type {SafeWorkspacePicker, SafeWorkspacePickerItem} from "../workspace.ts";
import {
  PickerWindow,
  pickerScopeBarVisible,
  type PickerInputMethod,
  type PickerScope
} from "./PickerWindow.tsx";

function preferredPickerWidth(viewportWidth: number): number {
  return Math.max(42, Math.min(86, viewportWidth - 2));
}

export function workspacePickerScopes(picker: SafeWorkspacePicker): readonly PickerScope[] {
  if (picker.scopes === undefined || picker.scopes.length <= 1) return [];
  return [{label: safeInlineText("ALL"), count: picker.items.length}, ...picker.scopes];
}

export function workspacePickerScopeBarVisible(
  picker: SafeWorkspacePicker,
  viewportWidth: number,
  viewportHeight: number
): boolean {
  return pickerScopeBarVisible({
    scopes: workspacePickerScopes(picker),
    label: picker.scopeLabel ?? "Scope",
    viewportWidth,
    viewportHeight,
    preferredWidth: preferredPickerWidth(viewportWidth)
  });
}

export function WorkspacePickerWindow(props: {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly picker: SafeWorkspacePicker;
  readonly query: string;
  readonly items: readonly SafeWorkspacePickerItem[];
  readonly activeScopeId?: string;
  readonly selectedId?: string;
  readonly truncated: boolean;
  readonly focused: boolean;
  readonly inputMethod: PickerInputMethod;
  readonly onInput: (value: string) => void;
  readonly onFocus: () => void;
  readonly onInputMethodChange: (method: PickerInputMethod) => void;
  readonly onMove: (id: string) => void;
  readonly onSelect: (id: string) => void;
  readonly onScopeChange: (id: string | undefined) => void;
  readonly onCancel: () => void;
}) {
  const scopes = workspacePickerScopes(props.picker);
  return (
    <PickerWindow
      colors={props.colors}
      viewportWidth={props.viewportWidth}
      viewportHeight={props.viewportHeight}
      preferredWidth={preferredPickerWidth(props.viewportWidth)}
      title={props.picker.title}
      query={props.query}
      placeholder={props.picker.placeholder}
      focused={props.focused}
      items={props.items.map(item => ({
        id: item.id,
        value: item,
        title: item.title,
        description: item.description,
        category: item.category,
        categoryId: item.scopeId ?? item.category,
        badge: item.badge
      }))}
      selectedId={props.selectedId}
      truncated={props.truncated}
      instruction={props.picker.instruction}
      emptyMessage={props.picker.emptyMessage}
      inputMethod={props.inputMethod}
      scopeLabel={props.picker.scopeLabel}
      scopes={scopes.length === 0 ? undefined : scopes}
      activeScopeId={props.activeScopeId}
      showItemsWithoutQuery
      onInput={props.onInput}
      onFocus={props.onFocus}
      onInputMethodChange={props.onInputMethodChange}
      onMove={props.onMove}
      onSelect={props.onSelect}
      onScopeChange={props.onScopeChange}
      onCancel={props.onCancel}
    />
  );
}
