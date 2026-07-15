import type {MouseEvent, ScrollBoxRenderable} from "@opentui/core";
import {MacOSScrollAccel, TextAttributes} from "@opentui/core";
import {useEffect, useMemo, useRef, useState} from "react";

import {readableColor} from "../color.ts";
import type {FocusTarget} from "../model.ts";
import type {Palette} from "../theme.ts";
import type {TreeKind, TreeRow} from "../tree.ts";

export interface JsonTreeResolvedColors {
  readonly normalForeground: string;
  readonly activeForeground: string;
  readonly normalSeparator: string;
  readonly activeSeparator: string;
  readonly normalMatchedLabel: string;
  readonly activeMatchedLabel: string;
  readonly activeMatchLabel: string;
  readonly normalValues: Readonly<Record<TreeKind, string>>;
  readonly activeValues: Readonly<Record<TreeKind, string>>;
}

export function resolveJsonTreeColors(colors: Palette): JsonTreeResolvedColors {
  const normalBackgrounds = [colors.panel, colors.background];
  const activeBackgrounds = [colors.selection, ...normalBackgrounds];
  const activeForeground = readableColor(
    colors.selectionText,
    colors.text,
    activeBackgrounds
  );
  const candidates: Readonly<Record<TreeKind, string>> = {
    string: colors.success,
    number: colors.accentSecondary,
    boolean: colors.warning,
    null: colors.textMuted,
    array: colors.accent,
    object: colors.accent
  };
  const valueColors = (backgrounds: readonly string[], fallback: string): Readonly<Record<TreeKind, string>> => ({
    string: readableColor(candidates.string, fallback, backgrounds),
    number: readableColor(candidates.number, fallback, backgrounds),
    boolean: readableColor(candidates.boolean, fallback, backgrounds),
    null: readableColor(candidates.null, fallback, backgrounds),
    array: readableColor(candidates.array, fallback, backgrounds),
    object: readableColor(candidates.object, fallback, backgrounds)
  });

  return {
    normalForeground: colors.text,
    activeForeground,
    normalSeparator: colors.textMuted,
    activeSeparator: activeForeground,
    normalMatchedLabel: readableColor(colors.accentSecondary, colors.text, normalBackgrounds),
    activeMatchedLabel: readableColor(colors.accentSecondary, activeForeground, activeBackgrounds),
    activeMatchLabel: readableColor(colors.warning, activeForeground, activeBackgrounds),
    normalValues: valueColors(normalBackgrounds, colors.text),
    activeValues: valueColors(activeBackgrounds, activeForeground)
  };
}

export interface JsonTreeProps {
  readonly colors: Palette;
  readonly rows: readonly TreeRow[];
  readonly selectedId: string;
  readonly focus: FocusTarget;
  readonly matchedIds?: ReadonlySet<string>;
  readonly activeMatchId?: string;
  readonly onFocus: () => void;
  readonly onSelect: (id: string) => void;
  readonly onToggle: (row: TreeRow) => void;
}

export function JsonTree(props: JsonTreeProps) {
  const scroll = useRef<ScrollBoxRenderable | null>(null);
  const [hovered, setHovered] = useState<string | undefined>();
  const scrollAcceleration = useMemo(() => new MacOSScrollAccel({maxMultiplier: 4}), []);
  const resolvedColors = useMemo(() => resolveJsonTreeColors(props.colors), [props.colors]);
  const selectedIndex = useMemo(
    () => Math.max(0, props.rows.findIndex(row => row.id === props.selectedId)),
    [props.rows, props.selectedId]
  );

  useEffect(() => {
    scroll.current?.scrollChildIntoView(`json-tree-row-${selectedIndex}`);
  }, [selectedIndex]);

  const stopAndToggle = (event: MouseEvent, row: TreeRow) => {
    event.stopPropagation();
    props.onFocus();
    props.onSelect(row.id);
    props.onToggle(row);
  };

  return (
    <scrollbox
      ref={value => { scroll.current = value; }}
      flexGrow={1}
      minHeight={0}
      focused={props.focus === "tree"}
      scrollAcceleration={scrollAcceleration}
      viewportOptions={{paddingRight: 1}}
      verticalScrollbarOptions={{
        visible: props.focus === "tree",
        trackOptions: {backgroundColor: props.colors.panel, foregroundColor: props.colors.borderActive}
      }}
      horizontalScrollbarOptions={{visible: false}}
      onMouseDown={props.onFocus}
    >
      {props.rows.map((row, index) => {
        const selected = row.id === props.selectedId;
        const matched = props.matchedIds?.has(row.id) === true;
        const activeMatch = row.id === props.activeMatchId;
        const active = selected || activeMatch || hovered === row.id;
        const caret = row.expandable ? row.expanded ? "▾" : "▸" : "·";
        const foreground = active ? resolvedColors.activeForeground : resolvedColors.normalForeground;
        const labelColor = selected
          ? resolvedColors.activeForeground
          : activeMatch
            ? resolvedColors.activeMatchLabel
            : matched
              ? active ? resolvedColors.activeMatchedLabel : resolvedColors.normalMatchedLabel
              : foreground;
        const separatorColor = active ? resolvedColors.activeSeparator : resolvedColors.normalSeparator;
        const valueColor = selected
          ? resolvedColors.activeForeground
          : active
            ? resolvedColors.activeValues[row.kind]
            : resolvedColors.normalValues[row.kind];
        return (
          <box
            id={`json-tree-row-${index}`}
            key={row.id}
            flexDirection="row"
            width="100%"
            backgroundColor={active ? props.colors.selection : undefined}
            onMouseOver={() => setHovered(row.id)}
            onMouseOut={() => setHovered(current => current === row.id ? undefined : current)}
            onMouseDown={() => {
              props.onFocus();
              props.onSelect(row.id);
            }}
          >
            <text fg={row.expandable ? props.colors.accent : props.colors.textMuted} onMouseDown={event => stopAndToggle(event, row)}>
              {`${"  ".repeat(row.depth)}${caret} `}
            </text>
            <text fg={foreground} wrapMode="none">
              <span style={{attributes: selected || matched ? TextAttributes.BOLD : undefined, fg: labelColor}}>{row.label}</span>
              <span style={{fg: separatorColor}}>{": "}</span>
              <span style={{fg: valueColor}}>{row.preview}</span>
            </text>
          </box>
        );
      })}
    </scrollbox>
  );
}
