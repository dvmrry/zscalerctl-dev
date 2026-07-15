import {TextAttributes} from "@opentui/core";

import type {Palette} from "../theme.ts";

export function Welcome(props: {readonly colors: Palette; readonly compact: boolean; readonly workspaceLabel: string}) {
  if (props.compact) {
    return (
      <box flexDirection="column" marginBottom={1} paddingLeft={1}>
        <text fg={props.colors.text} attributes={TextAttributes.BOLD}>
          <span style={{fg: props.colors.accent}}>◆ </span>zscalerctl <span style={{fg: props.colors.textMuted}}>OpenTUI lab</span>
        </text>
        <text fg={props.colors.textMuted}><span style={{fg: props.colors.success}}>●</span> {props.workspaceLabel} · tenant read-only · no model</text>
      </box>
    );
  }
  return (
    <box
      flexDirection="column"
      paddingLeft={2}
      paddingTop={1}
      paddingBottom={1}
      marginBottom={1}
    >
      <text fg={props.colors.text} attributes={TextAttributes.BOLD}>
        <span style={{fg: props.colors.accent}}>◆ </span>zscalerctl <span style={{fg: props.colors.textMuted}}>OpenTUI lab</span>
      </text>
      <text fg={props.colors.textMuted}>Agentic tenant explorer with a structured data workspace.</text>
      <box flexDirection="row" gap={2}>
        <text fg={props.colors.success}>● tenant read-only</text>
        <text fg={props.colors.textMuted}>{props.workspaceLabel}</text>
        <text fg={props.colors.warning}>no model attached</text>
      </box>
      <text fg={props.colors.textMuted}>
        {props.workspaceLabel === "fixture"
          ? "/demo reloads data · Ctrl+F searches · Ctrl+O opens the inspector"
          : "/catalog browses resources · Ctrl+F searches data · Ctrl+O inspects"}
      </text>
    </box>
  );
}
