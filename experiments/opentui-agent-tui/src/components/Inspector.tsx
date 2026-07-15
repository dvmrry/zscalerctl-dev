import {TextAttributes} from "@opentui/core";

import type {FocusTarget} from "../model.ts";
import {safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {formatBreadcrumb, type TreeRow} from "../tree.ts";
import {JsonTree} from "./JsonTree.tsx";
import {OverlayBackdrop} from "./Overlay.tsx";
import {Pane} from "./Pane.tsx";

export interface InspectorProps {
  readonly colors: Palette;
  readonly width: number;
  readonly rows: readonly TreeRow[];
  readonly selectedId: string;
  readonly selectedRow: TreeRow;
  readonly focus: FocusTarget;
  readonly matchedIds?: ReadonlySet<string>;
  readonly activeMatchId?: string;
  readonly onClose: () => void;
  readonly onFocusTree: () => void;
  readonly onSelect: (id: string) => void;
  readonly onToggle: (row: TreeRow) => void;
}

export function Inspector(props: InspectorProps) {
  return (
    <OverlayBackdrop
      layer="dialog"
      align="end"
      dim={72}
      contentWidth={props.width}
      contentHeight="100%"
      onDismiss={props.onClose}
    >
      <box
        width="100%"
        height="100%"
        backgroundColor={props.colors.background}
        paddingTop={1}
        paddingBottom={1}
        paddingLeft={1}
        paddingRight={1}
      >
        <box flexDirection="row" justifyContent="space-between" height={1} marginBottom={1}>
          <text fg={props.colors.text} attributes={TextAttributes.BOLD}>JSON Inspector</text>
          <text fg={props.colors.textMuted}>Esc · click outside</text>
        </box>
        <Pane
          colors={props.colors}
          title="Sanitized result"
          subtitle={safeInlineText(formatBreadcrumb(props.rows, props.selectedRow), Math.max(20, props.width - 28))}
          active
          flexGrow={1}
          minHeight={8}
        >
          <JsonTree
            colors={props.colors}
            rows={props.rows}
            selectedId={props.selectedId}
            focus={props.focus}
            matchedIds={props.matchedIds}
            activeMatchId={props.activeMatchId}
            onFocus={props.onFocusTree}
            onSelect={props.onSelect}
            onToggle={props.onToggle}
          />
        </Pane>
        <box height={1} />
        <Pane colors={props.colors} title="Selection" subtitle={props.selectedRow.kind}>
          <text fg={props.colors.textMuted} wrapMode="word">{props.selectedRow.preview}</text>
        </Pane>
      </box>
    </OverlayBackdrop>
  );
}
