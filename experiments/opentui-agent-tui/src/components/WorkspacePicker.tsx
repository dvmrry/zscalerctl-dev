import type {Palette} from "../theme.ts";
import type {WorkspacePicker, WorkspacePickerItem} from "../workspace.ts";
import {PickerWindow, type PickerInputMethod} from "./PickerWindow.tsx";

export function WorkspacePickerWindow(props: {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly picker: WorkspacePicker;
  readonly query: string;
  readonly items: readonly WorkspacePickerItem[];
  readonly selectedId?: string;
  readonly truncated: boolean;
  readonly focused: boolean;
  readonly inputMethod: PickerInputMethod;
  readonly onInput: (value: string) => void;
  readonly onFocus: () => void;
  readonly onInputMethodChange: (method: PickerInputMethod) => void;
  readonly onMove: (id: string) => void;
  readonly onSelect: (id: string) => void;
  readonly onCancel: () => void;
}) {
  return (
    <PickerWindow
      colors={props.colors}
      viewportWidth={props.viewportWidth}
      viewportHeight={props.viewportHeight}
      preferredWidth={Math.max(42, Math.min(86, props.viewportWidth - 2))}
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
        categoryId: item.category,
        badge: item.badge
      }))}
      selectedId={props.selectedId}
      truncated={props.truncated}
      instruction={props.picker.instruction}
      emptyMessage={props.picker.emptyMessage}
      inputMethod={props.inputMethod}
      showItemsWithoutQuery
      onInput={props.onInput}
      onFocus={props.onFocus}
      onInputMethodChange={props.onInputMethodChange}
      onMove={props.onMove}
      onSelect={props.onSelect}
      onCancel={props.onCancel}
    />
  );
}
