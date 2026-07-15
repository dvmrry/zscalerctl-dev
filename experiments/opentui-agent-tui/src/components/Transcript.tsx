import {TextAttributes} from "@opentui/core";

import type {FocusTarget, TranscriptEntry} from "../model.ts";
import {toneColor} from "../model.ts";
import type {Palette} from "../theme.ts";
import {Welcome} from "./Welcome.tsx";

export interface TranscriptProps {
  readonly colors: Palette;
  readonly entries: readonly TranscriptEntry[];
  readonly focus: FocusTarget;
  readonly compact: boolean;
  readonly workspaceLabel: string;
  readonly onFocus: () => void;
}

function Entry(props: {readonly colors: Palette; readonly entry: TranscriptEntry}) {
  if (props.entry.role === "user") {
    return (
      <box
        flexDirection="column"
        flexShrink={0}
        border={["left"]}
        borderColor={props.colors.accentSecondary}
        backgroundColor={props.colors.panel}
        paddingTop={1}
        paddingBottom={1}
        paddingLeft={2}
        paddingRight={2}
        marginTop={1}
      >
        {props.entry.body.map((line, index) => (
          <text key={`${props.entry.id}:${index}`} fg={props.colors.text} wrapMode="word">{line}</text>
        ))}
        <text fg={props.colors.textMuted}>You</text>
      </box>
    );
  }

  const markerColor = toneColor(props.entry.tone, props.colors);
  const marker = props.entry.role === "assistant" ? "◆" : "●";
  return (
    <box flexDirection="column" marginTop={1} flexShrink={0}>
      <box flexDirection="row" gap={1}>
        <text fg={markerColor} attributes={TextAttributes.BOLD}>{marker}</text>
        {props.entry.title === undefined ? null : (
          <text fg={props.colors.text} attributes={TextAttributes.BOLD}>{props.entry.title}</text>
        )}
      </box>
      <box flexDirection="column" paddingLeft={3}>
        {props.entry.body.map((line, index) => (
          <text key={`${props.entry.id}:${index}`} fg={props.colors.textMuted} wrapMode="word">{line}</text>
        ))}
        {props.entry.data === undefined ? null : (
          <text fg={props.colors.accent}>↳ Structured result · Ctrl+O inspect · Ctrl+F find</text>
        )}
      </box>
    </box>
  );
}

export function Transcript(props: TranscriptProps) {
  return (
    <box flexDirection="column" flexGrow={1} minHeight={0} onMouseDown={props.onFocus}>
      <scrollbox
        flexGrow={1}
        minHeight={0}
        stickyScroll
        stickyStart="bottom"
        focused={props.focus === "transcript"}
        viewportOptions={{paddingRight: props.focus === "transcript" ? 1 : 0}}
        verticalScrollbarOptions={{
          visible: props.focus === "transcript",
          trackOptions: {backgroundColor: props.colors.background, foregroundColor: props.colors.borderActive}
        }}
      >
        <Welcome colors={props.colors} compact={props.compact} workspaceLabel={props.workspaceLabel} />
        {props.entries.map(entry => <Entry key={entry.id} colors={props.colors} entry={entry} />)}
      </scrollbox>
    </box>
  );
}
