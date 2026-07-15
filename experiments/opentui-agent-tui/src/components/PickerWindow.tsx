import {MacOSScrollAccel, TextAttributes, type MouseEvent, type ScrollBoxRenderable} from "@opentui/core";
import {useEffect, useMemo, useRef} from "react";

import {safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {FloatingWindow} from "./Overlay.tsx";

export type PickerInputMethod = "keyboard" | "mouse";

export interface PickerItem<T> {
  readonly id: string;
  readonly value: T;
  readonly title: string;
  readonly description?: string;
  readonly category?: string;
  readonly categoryId?: string;
  readonly badge?: string;
  readonly disabled?: boolean;
}

export interface PickerAction<T> {
  readonly id: string;
  readonly shortcut: string;
  readonly compactShortcut?: string;
  readonly label: string;
  readonly compactLabel?: string;
  readonly disabled?: (item: PickerItem<T> | undefined) => boolean;
  readonly onTrigger: (item: PickerItem<T> | undefined) => void;
}

export interface PickerWindowProps<T> {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly preferredWidth: number;
  readonly title: string;
  readonly query: string;
  readonly placeholder: string;
  readonly focused: boolean;
  readonly items: readonly PickerItem<T>[];
  readonly selectedId?: string;
  readonly truncated?: boolean;
  readonly instruction: string;
  readonly emptyMessage: string;
  readonly inputMethod: PickerInputMethod;
  readonly showItemsWithoutQuery?: boolean;
  readonly actions?: readonly PickerAction<T>[];
  readonly onInput: (value: string) => void;
  readonly onFocus: () => void;
  readonly onInputMethodChange: (method: PickerInputMethod) => void;
  readonly onMove: (id: string) => void;
  readonly onSelect: (id: string) => void;
  readonly onCancel: () => void;
}

function categoryKey<T>(item: PickerItem<T>): string | undefined {
  if (item.category === undefined) return undefined;
  return item.categoryId ?? item.category;
}

export function PickerWindow<T>(props: PickerWindowProps<T>) {
  const scroll = useRef<ScrollBoxRenderable | null>(null);
  const lastMousePosition = useRef<{readonly x: number; readonly y: number} | undefined>(undefined);
  const scrollAcceleration = useMemo(() => new MacOSScrollAccel({maxMultiplier: 4}), []);
  const selectedIndex = Math.max(0, props.items.findIndex(item => item.id === props.selectedId));
  const selectedItem = props.items[selectedIndex];
  const windowWidth = Math.min(props.preferredWidth, Math.max(1, props.viewportWidth - 2));
  const compactWidth = windowWidth < 62;
  const veryCompactWidth = windowWidth < 44;
  const showResults = props.viewportHeight >= 10;
  const categoryRows = props.items.reduce((count, item, index) => {
    const key = categoryKey(item);
    if (key === undefined) return count;
    return count + (index === 0 || key !== categoryKey(props.items[index - 1]!) ? 1 : 0);
  }, 0);
  const contentRows = props.items.length * 2 + categoryRows;
  const preferredHeight = showResults
    ? Math.min(20, Math.max(9, Math.min(14, Math.max(2, contentRows)) + 6))
    : 5;
  const visibleItemEstimate = Math.max(1, Math.floor((preferredHeight - 6) / 2));
  const total = props.items.length;
  const status = props.query.trim().length === 0
    ? props.showItemsWithoutQuery === true && total > 0
      ? `${selectedIndex + 1}/${total}${props.truncated === true ? "+" : ""}`
      : "type to search"
    : total === 0
      ? "no matches"
      : `${selectedIndex + 1}/${total}${props.truncated === true ? "+" : ""}`;
  const visibleActions = compactWidth
    ? (props.actions ?? []).slice(0, veryCompactWidth ? 1 : 2)
    : props.actions ?? [];

  useEffect(() => {
    if (selectedItem !== undefined) scroll.current?.scrollChildIntoView(`picker-item-${selectedIndex}`);
  }, [selectedIndex, selectedItem]);

  const handleMouseMove = (item: PickerItem<T>, event: MouseEvent) => {
    const previous = lastMousePosition.current;
    if (previous?.x === event.x && previous.y === event.y) return;
    lastMousePosition.current = {x: event.x, y: event.y};
    if (props.inputMethod !== "mouse") props.onInputMethodChange("mouse");
    if (!item.disabled) props.onMove(item.id);
  };

  return (
    <FloatingWindow
      colors={props.colors}
      viewportWidth={props.viewportWidth}
      viewportHeight={props.viewportHeight}
      preferredWidth={props.preferredWidth}
      preferredHeight={preferredHeight}
      placement="top-center"
      layer="utility"
      title={` ${safeInlineText(props.title, Math.max(4, windowWidth - 8))} `}
      bottomTitle={` ${status} `}
      onFocus={props.onFocus}
    >
      <box flexGrow={1} minHeight={0} flexDirection="column" paddingLeft={1} paddingRight={1}>
        <box height={1} flexShrink={0} flexDirection="row" gap={1}>
          <text fg={props.colors.accent}>⌕</text>
          <input
            id="picker-query"
            value={props.query}
            focused={props.focused}
            flexGrow={1}
            maxLength={120}
            placeholder={safeInlineText(props.placeholder, 180)}
            backgroundColor={props.colors.surface}
            focusedBackgroundColor={props.colors.surfaceFocus}
            textColor={props.colors.text}
            focusedTextColor={props.colors.text}
            placeholderColor={props.colors.textMuted}
            selectionBg={props.colors.selection}
            selectionFg={props.colors.selectionText}
            onInput={value => {
              props.onInputMethodChange("keyboard");
              props.onInput(value);
            }}
            onSubmit={() => {
              if (selectedItem !== undefined && !selectedItem.disabled) props.onSelect(selectedItem.id);
            }}
          />
        </box>

        {showResults ? (
          <scrollbox
            ref={value => { scroll.current = value; }}
            flexGrow={1}
            minHeight={2}
            marginTop={1}
            scrollAcceleration={scrollAcceleration}
            viewportOptions={{paddingRight: 1}}
            verticalScrollbarOptions={{
              visible: total > visibleItemEstimate,
              trackOptions: {backgroundColor: props.colors.panelRaised, foregroundColor: props.colors.borderActive}
            }}
            horizontalScrollbarOptions={{visible: false}}
          >
            {props.query.trim().length === 0 && props.showItemsWithoutQuery !== true ? (
              <box height={2} paddingLeft={1}>
                <text fg={props.colors.textMuted}>{safeInlineText(props.instruction, 500)}</text>
              </box>
            ) : total === 0 ? (
              <box height={2} paddingLeft={1}>
                <text fg={props.colors.warning}>{safeInlineText(props.emptyMessage, 500)}</text>
              </box>
            ) : props.items.map((item, index) => {
              const active = item.id === props.selectedId || (props.selectedId === undefined && index === 0);
              const currentCategory = categoryKey(item);
              const previousCategory = index === 0 ? undefined : categoryKey(props.items[index - 1]!);
              const showCategory = item.category !== undefined && (index === 0 || currentCategory !== previousCategory);
              const foreground = active ? props.colors.selectionText : item.disabled ? props.colors.textMuted : props.colors.text;
              return (
                <box key={item.id} flexDirection="column" flexShrink={0}>
                  {showCategory ? (
                    <box height={1} flexShrink={0} paddingLeft={1} marginTop={index === 0 ? 0 : 1}>
                      <text fg={props.colors.accent} attributes={TextAttributes.BOLD} wrapMode="none">
                        {safeInlineText(item.category!, Math.max(4, windowWidth - 6))}
                      </text>
                    </box>
                  ) : null}
                  <box
                    id={`picker-item-${index}`}
                    height={2}
                    flexShrink={0}
                    flexDirection="column"
                    paddingLeft={1}
                    paddingRight={1}
                    backgroundColor={active ? props.colors.selection : undefined}
                    onMouseMove={event => handleMouseMove(item, event)}
                    onMouseDown={() => {
                      if (props.inputMethod !== "mouse") props.onInputMethodChange("mouse");
                      if (!item.disabled) props.onMove(item.id);
                    }}
                    onMouseUp={() => {
                      if (!item.disabled) props.onSelect(item.id);
                    }}
                  >
                    <box height={1} flexDirection="row" justifyContent="space-between">
                      <text fg={foreground} attributes={active ? TextAttributes.BOLD : undefined} wrapMode="none">
                        {safeInlineText(item.title, Math.max(4, windowWidth - 18))}
                      </text>
                      {item.badge === undefined ? null : (
                        <text fg={active ? props.colors.selectionText : props.colors.textMuted}>{safeInlineText(item.badge, 80)}</text>
                      )}
                    </box>
                    <text fg={active ? props.colors.selectionText : props.colors.textMuted} wrapMode="none">
                      {safeInlineText(item.description ?? "", Math.max(4, windowWidth - 6))}
                    </text>
                  </box>
                </box>
              );
            })}
          </scrollbox>
        ) : null}

        <box height={1} flexShrink={0} flexDirection="row" justifyContent="space-between" marginTop={showResults ? 1 : 0}>
          <box flexDirection="row" gap={compactWidth ? 1 : 2}>
            {visibleActions.map(action => {
              const disabled = action.disabled?.(selectedItem) ?? false;
              return (
                <text
                  key={action.id}
                  fg={disabled ? props.colors.textMuted : props.colors.accent}
                  onMouseDown={() => {
                    if (!disabled) action.onTrigger(selectedItem);
                  }}
                >
                  {compactWidth ? action.compactShortcut ?? action.shortcut : action.shortcut}{" "}
                  {compactWidth ? action.compactLabel ?? action.label : action.label}
                </text>
              );
            })}
          </box>
          <text fg={props.colors.textMuted} onMouseDown={props.onCancel}>{veryCompactWidth ? "Esc" : "Esc cancel"}</text>
        </box>
      </box>
    </FloatingWindow>
  );
}
