import {MacOSScrollAccel, TextAttributes, type MouseEvent, type ScrollBoxRenderable} from "@opentui/core";
import {useEffect, useMemo, useRef} from "react";

import {safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {placeFloatingWindow} from "../overlay.ts";
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

export interface PickerScope {
  readonly id?: string;
  readonly label: string;
  readonly count: number;
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
  readonly scopeLabel?: string;
  readonly scopes?: readonly PickerScope[];
  readonly activeScopeId?: string;
  readonly onInput: (value: string) => void;
  readonly onFocus: () => void;
  readonly onInputMethodChange: (method: PickerInputMethod) => void;
  readonly onMove: (id: string) => void;
  readonly onSelect: (id: string) => void;
  readonly onScopeChange?: (id: string | undefined) => void;
  readonly onCancel: () => void;
}

function categoryKey<T>(item: PickerItem<T>): string | undefined {
  if (item.category === undefined) return undefined;
  return item.categoryId ?? item.category;
}

const GRAPHEME_SEGMENTER = new Intl.Segmenter(undefined, {granularity: "grapheme"});

export function fitPickerCellText(value: string, maximumWidth: number): string {
  const width = Math.max(1, Math.floor(maximumWidth));
  if (Bun.stringWidth(value) <= width) return value;
  if (width === 1) return "…";
  let output = "";
  let used = 0;
  for (const {segment} of GRAPHEME_SEGMENTER.segment(value)) {
    const segmentWidth = Bun.stringWidth(segment);
    if (used + segmentWidth + 1 > width) break;
    output += segment;
    used += segmentWidth;
  }
  return `${output}…`;
}

export function pickerScopeText(scope: PickerScope, availableWidth: number): string {
  const width = Math.max(1, Math.floor(availableWidth));
  const label = safeInlineText(scope.label, 80);
  const count = String(scope.count);
  const padded = ` ${label} ${count} `;
  if (Bun.stringWidth(padded) <= width) return padded;
  const compact = `${label} ${count}`;
  if (Bun.stringWidth(compact) <= width) return compact;
  const suffix = ` ${count}`;
  const suffixWidth = Bun.stringWidth(suffix);
  if (suffixWidth >= width) return fitPickerCellText(compact, width);
  return `${fitPickerCellText(label, width - suffixWidth)}${suffix}`;
}

export function pickerScopeRowCount(scopes: readonly PickerScope[], label: string, availableWidth: number): number {
  if (scopes.length === 0) return 0;
  const width = Math.max(1, availableWidth);
  const widths = [
    Bun.stringWidth(fitPickerCellText(label, width)),
    ...scopes.map(scope => Bun.stringWidth(pickerScopeText(scope, width)))
  ];
  let rows = 1;
  let used = 0;
  for (const itemWidth of widths) {
    const next = used === 0 ? itemWidth : used + 1 + itemWidth;
    if (used > 0 && next > width) {
      rows += 1;
      used = itemWidth;
    } else {
      used = next;
    }
  }
  return rows;
}

export function pickerScopeBarVisible(options: {
  readonly scopes: readonly PickerScope[];
  readonly label: string;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly preferredWidth: number;
}): boolean {
  if (options.scopes.length <= 1) return false;
  const maximum = placeFloatingWindow({
    viewportWidth: options.viewportWidth,
    viewportHeight: options.viewportHeight,
    preferredWidth: options.preferredWidth,
    preferredHeight: Number.MAX_SAFE_INTEGER,
    placement: "top-center"
  });
  const label = safeInlineText(options.label, 80).toUpperCase();
  const rows = pickerScopeRowCount(options.scopes, label, Math.max(1, maximum.width - 4));
  const minimumWindowHeight = 9 + rows + 1;
  return maximum.height >= minimumWindowHeight;
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
  const visibleScopes = props.scopes ?? [];
  const rawScopeLabel = safeInlineText(props.scopeLabel ?? "Scope", 80).toUpperCase();
  const scopeContentWidth = Math.max(1, windowWidth - 4);
  const showScopeBar = props.onScopeChange !== undefined && pickerScopeBarVisible({
    scopes: visibleScopes,
    label: rawScopeLabel,
    viewportWidth: props.viewportWidth,
    viewportHeight: props.viewportHeight,
    preferredWidth: props.preferredWidth
  });
  const scopeLabel = fitPickerCellText(rawScopeLabel, scopeContentWidth);
  const scopeTexts = visibleScopes.map(scope => pickerScopeText(scope, scopeContentWidth));
  const scopeRows = showScopeBar ? pickerScopeRowCount(visibleScopes, rawScopeLabel, scopeContentWidth) : 0;
  const scopeChromeRows = scopeRows === 0 ? 0 : scopeRows + 1;
  const categoryRows = props.items.reduce((count, item, index) => {
    const key = categoryKey(item);
    if (key === undefined) return count;
    return count + (index === 0 || key !== categoryKey(props.items[index - 1]!) ? 1 : 0);
  }, 0);
  const contentRows = props.items.length * 2 + categoryRows;
  const preferredHeight = showResults
    ? Math.min(23, Math.max(9 + scopeChromeRows, Math.min(14, Math.max(2, contentRows)) + 6 + scopeChromeRows))
    : 5 + scopeChromeRows;
  const visibleItemEstimate = Math.max(1, Math.floor((preferredHeight - 6 - scopeChromeRows) / 2));
  const total = props.items.length;
  const status = props.query.trim().length === 0
    ? props.showItemsWithoutQuery === true && total > 0
      ? `${selectedIndex + 1}/${total}${props.truncated === true ? "+" : ""}`
      : "type to search"
    : total === 0
      ? "no matches"
      : `${selectedIndex + 1}/${total}${props.truncated === true ? "+" : ""}`;
  const activeScope = visibleScopes.find(scope => scope.id === props.activeScopeId);
  const scopedStatus = showScopeBar && activeScope !== undefined ? `${activeScope.label} · ${status}` : status;
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
      bottomTitle={` ${scopedStatus} `}
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

        {showScopeBar ? (
          <box
            height={scopeRows}
            flexShrink={0}
            flexDirection="row"
            flexWrap="wrap"
            columnGap={1}
            rowGap={0}
            marginTop={1}
          >
            <text fg={props.colors.textMuted} attributes={TextAttributes.BOLD} wrapMode="none">{scopeLabel}</text>
            {visibleScopes.map((scope, index) => {
              const active = scope.id === props.activeScopeId;
              return (
                <text
                  id={`picker-scope-${index}`}
                  key={`picker-scope-${index}`}
                  fg={active ? props.colors.selectionText : props.colors.text}
                  bg={active ? props.colors.selection : props.colors.panelRaised}
                  attributes={active ? TextAttributes.BOLD : undefined}
                  wrapMode="none"
                  onMouseDown={() => {
                    props.onInputMethodChange("mouse");
                    props.onScopeChange?.(scope.id);
                  }}
                >
                  {scopeTexts[index]}
                </text>
              );
            })}
          </box>
        ) : null}

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
            {showScopeBar ? <text fg={props.colors.textMuted}>{veryCompactWidth ? "Tab" : "Tab product"}</text> : null}
          </box>
          <text fg={props.colors.textMuted} onMouseDown={props.onCancel}>{veryCompactWidth ? "Esc" : "Esc cancel"}</text>
        </box>
      </box>
    </FloatingWindow>
  );
}
