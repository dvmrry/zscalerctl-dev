import type {MouseEvent, ScrollBoxRenderable} from "@opentui/core";
import {MacOSScrollAccel, TextAttributes} from "@opentui/core";
import {useEffect, useMemo, useRef, useState} from "react";

import type {FocusTarget} from "../model.ts";
import type {Palette} from "../theme.ts";
import type {TreeKind, TreeRow} from "../tree.ts";

function kindColor(kind: TreeKind, colors: Palette): string {
  switch (kind) {
    case "string": return colors.success;
    case "number": return colors.accentSecondary;
    case "boolean": return colors.warning;
    case "null": return colors.textMuted;
    case "array":
    case "object": return colors.accent;
  }
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
            <text fg={selected ? props.colors.selectionText : props.colors.text} wrapMode="none">
              <span style={{attributes: selected || matched ? TextAttributes.BOLD : undefined, fg: selected ? props.colors.selectionText : activeMatch ? props.colors.warning : matched ? props.colors.accentSecondary : undefined}}>{row.label}</span>
              <span style={{fg: selected ? props.colors.selectionText : props.colors.textMuted}}>{": "}</span>
              <span style={{fg: selected ? props.colors.selectionText : kindColor(row.kind, props.colors)}}>{row.preview}</span>
            </text>
          </box>
        );
      })}
    </scrollbox>
  );
}
