import {TextAttributes} from "@opentui/core";
import {useEffect, useState} from "react";

import type {ContextState, FocusTarget} from "../model.ts";
import {safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import type {ArrayOrder, TreeRow} from "../tree.ts";
import {JsonTree} from "./JsonTree.tsx";

const ACTIVITY_FRAMES = ["삼", "이", "일"] as const;

export interface ContextRailProps {
  readonly colors: Palette;
  readonly context: ContextState;
  readonly rows: readonly TreeRow[];
  readonly selectedId: string;
  readonly selectedRow: TreeRow;
  readonly focus: FocusTarget;
  readonly overlay: boolean;
  readonly compact: boolean;
  readonly arrayOrder: ArrayOrder;
  readonly searchSummary?: string;
  readonly matchedIds?: ReadonlySet<string>;
  readonly activeMatchId?: string;
  readonly onClose: () => void;
  readonly onFocusTree: () => void;
  readonly onSelect: (id: string) => void;
  readonly onToggle: (row: TreeRow) => void;
  readonly onToggleOrder: () => void;
  readonly onSearch: () => void;
  readonly onInspect: () => void;
}

export function ContextRail(props: ContextRailProps) {
  const [scopeOpen, setScopeOpen] = useState(true);
  const [activityFrame, setActivityFrame] = useState(0);

  useEffect(() => {
    if (props.context.operation.status !== "running") return;
    const timer = setInterval(() => setActivityFrame(value => (value + 1) % ACTIVITY_FRAMES.length), 120);
    return () => clearInterval(timer);
  }, [props.context.operation.status]);

  const operation = props.context.operation;
  const operationLabel = operation.status === "running"
    ? `${ACTIVITY_FRAMES[activityFrame]} ${operation.label}`
    : operation.label;
  const selectedLabel = safeInlineText(`${props.selectedRow.label} · ${props.selectedRow.kind}`, 30);
  const themeLabel = safeInlineText(`${props.colors.name} · ${props.colors.mode}`, 15);
  const operationColor = operation.status === "error"
    ? props.colors.danger
    : operation.status === "running"
      ? props.colors.accent
      : operation.status === "complete"
        ? props.colors.success
        : props.colors.textMuted;
  const connectionColor = props.context.connection === "connected" || props.context.connection === "fixture"
    ? props.colors.success
    : props.context.connection === "error"
      ? props.colors.danger
      : props.colors.warning;
  const hasProgress = operation.current !== undefined && operation.total !== undefined;

  return (
    <box
      width={48}
      height="100%"
      flexDirection="column"
      backgroundColor={props.colors.panel}
      paddingLeft={2}
      paddingRight={2}
      paddingTop={1}
      paddingBottom={1}
    >
      <box flexDirection="row" justifyContent="space-between" height={1} flexShrink={0}>
        <text fg={props.colors.text} attributes={TextAttributes.BOLD}>Tenant workspace</text>
        <text fg={props.colors.textMuted} onMouseDown={props.overlay ? props.onClose : undefined}>
          {props.overlay ? "close" : "Ctrl+B"}
        </text>
      </box>
      <box height={1} flexShrink={0} flexDirection="row" justifyContent="space-between">
        <text fg={props.colors.textMuted}>
          <span style={{fg: connectionColor}}>●</span> {props.context.connection} · {props.context.authority}
        </text>
        <text fg={props.colors.accent}>{themeLabel}</text>
      </box>

      <box flexShrink={0} marginTop={props.compact ? 0 : 1}>
        <box
          height={1}
          flexDirection="row"
          justifyContent="space-between"
          onMouseDown={() => setScopeOpen(value => !value)}
        >
          <text fg={props.colors.text} attributes={TextAttributes.BOLD}>{scopeOpen ? "▾" : "▸"} Scope</text>
          <text fg={props.colors.textMuted}>{props.context.records} records</text>
        </box>
        {scopeOpen ? (
          <box paddingLeft={2}>
            <text fg={props.colors.textMuted}>resource  <span style={{fg: props.colors.text}}>{props.context.scope}</span></text>
            {props.compact ? null : (
              <text fg={props.colors.textMuted}>transport <span style={{fg: props.colors.text}}>{props.context.transport}</span></text>
            )}
            <text fg={props.colors.textMuted}>selected  <span style={{fg: props.colors.accent}}>{selectedLabel}</span></text>
          </box>
        ) : null}
      </box>

      <box
        flexGrow={1}
        flexShrink={1}
        minHeight={props.compact ? 7 : 10}
        marginTop={props.compact ? 0 : 1}
        border={["left"]}
        borderColor={props.focus === "tree" ? props.colors.borderActive : props.colors.border}
        paddingLeft={1}
      >
        <box height={1} flexShrink={0} flexDirection="row" justifyContent="space-between" onMouseDown={props.onInspect}>
          <text fg={props.colors.text} attributes={TextAttributes.BOLD}>Data</text>
          <text fg={props.colors.textMuted}>{props.selectedRow.kind} · Ctrl+O</text>
        </box>
        <box height={1} flexShrink={0} flexDirection="row" justifyContent="space-between">
          <text fg={props.colors.textMuted} onMouseDown={props.onSearch}>{props.searchSummary ?? "Ctrl+F find"}</text>
          <text fg={props.colors.accent} onMouseDown={props.onToggleOrder}>{props.arrayOrder} order · S</text>
        </box>
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
      </box>

      <box height={1} flexShrink={0} marginTop={1} flexDirection="row" justifyContent="space-between">
        <text fg={operationColor} wrapMode="none">{safeInlineText(operationLabel, hasProgress ? 29 : 22)}</text>
        <text fg={props.colors.textMuted}>
          {!hasProgress
            ? safeInlineText(props.context.effects, 18)
            : `${operation.current}/${operation.total}`}
        </text>
      </box>
    </box>
  );
}
