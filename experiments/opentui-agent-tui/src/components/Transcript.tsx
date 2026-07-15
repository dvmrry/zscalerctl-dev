import {TextAttributes, type MouseEvent} from "@opentui/core";

import type {
  ContextState,
  FocusTarget,
  TranscriptActionID,
  TranscriptBlock,
  TranscriptEntry,
  TranscriptEvidence
} from "../model.ts";
import {toneColor} from "../model.ts";
import {fitCellText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {OperationScene} from "./OperationScene.tsx";
import {Welcome} from "./Welcome.tsx";

export interface TranscriptProps {
  readonly colors: Palette;
  readonly entries: readonly TranscriptEntry[];
  readonly focus: FocusTarget;
  readonly compact: boolean;
  readonly artwork: boolean;
  readonly availableWidth: number;
  readonly workspaceLabel: string;
  readonly context: ContextState;
  readonly operationSceneVisible: boolean;
  readonly interactive: boolean;
  readonly onFocus: () => void;
  readonly onAction: (entry: TranscriptEntry, action: TranscriptActionID) => void;
  readonly onEvidence: (entry: TranscriptEntry, evidence: TranscriptEvidence) => void;
  readonly onPinEvidence: (entry: TranscriptEntry, evidence: TranscriptEvidence) => void;
}

function invoke(event: MouseEvent, action: () => void): void {
  event.stopPropagation();
  action();
}

function Block(props: {
  readonly colors: Palette;
  readonly entry: TranscriptEntry;
  readonly block: TranscriptBlock;
  readonly availableWidth: number;
  readonly interactive: boolean;
  readonly onAction: TranscriptProps["onAction"];
  readonly onEvidence: TranscriptProps["onEvidence"];
  readonly onPinEvidence: TranscriptProps["onPinEvidence"];
}) {
  switch (props.block.kind) {
    case "text":
      return (
        <box flexDirection="column">
          {props.block.lines.map((line, index) => (
            <text key={index} fg={props.entry.role === "user" ? props.colors.text : props.colors.textMuted} wrapMode="word">{line}</text>
          ))}
        </box>
      );
    case "metrics":
      return (
        <box flexDirection="row" flexWrap="wrap" columnGap={2}>
          {props.block.items.map(item => (
            <text key={item.label} fg={props.colors.textMuted}>
              {item.label.toLowerCase()} <span style={{fg: toneColor(item.tone, props.colors)}}>{item.value}</span>
            </text>
          ))}
        </box>
      );
    case "facets":
      return (
        <box flexDirection="column">
          {props.block.items.map(item => (
            <text key={item.label} fg={props.colors.textMuted} wrapMode="none">
              {item.label.toLowerCase()}  <span style={{fg: props.colors.text}}>
                {fitCellText(item.values.map(value => `${value.label} ${value.count}`).join(" · "), Math.max(8, props.availableWidth - item.label.length - 5))}
              </span>
            </text>
          ))}
        </box>
      );
    case "evidence":
      return (
        <box flexDirection="column" marginTop={1}>
          {props.block.items.map((item, index) => (
            <box key={JSON.stringify(item.path)} flexDirection="row" width="100%" gap={1}>
              <text
                id={`transcript-evidence-${props.entry.id}-${index}`}
                fg={props.interactive ? props.colors.accentSecondary : props.colors.textMuted}
                flexGrow={1}
                wrapMode="none"
                onMouseDown={props.interactive
                  ? event => invoke(event, () => props.onEvidence(props.entry, item))
                  : undefined}
              >
                {fitCellText(`↳ ${item.label} · ${item.detail}`, Math.max(8, props.availableWidth - 10))}
              </text>
              <text
                id={`transcript-pin-${props.entry.id}-${index}`}
                fg={props.interactive ? props.colors.accent : props.colors.textMuted}
                onMouseDown={props.interactive
                  ? event => invoke(event, () => props.onPinEvidence(props.entry, item))
                  : undefined}
              >
                + pin
              </text>
            </box>
          ))}
        </box>
      );
    case "actions":
      return (
        <box flexDirection="row" gap={2} marginTop={1}>
          {props.block.items.map(item => (
            <text
              id={`transcript-action-${props.entry.id}-${item.id}`}
              key={item.id}
              fg={props.interactive ? props.colors.accent : props.colors.textMuted}
              onMouseDown={props.interactive
                ? event => invoke(event, () => props.onAction(props.entry, item.id))
                : undefined}
            >
              [{item.label}]<span style={{fg: props.colors.textMuted}}>{item.hint === undefined ? "" : ` ${item.hint}`}</span>
            </text>
          ))}
        </box>
      );
  }
}

function Entry(props: {
  readonly colors: Palette;
  readonly entry: TranscriptEntry;
  readonly availableWidth: number;
  readonly interactive: boolean;
  readonly onAction: TranscriptProps["onAction"];
  readonly onEvidence: TranscriptProps["onEvidence"];
  readonly onPinEvidence: TranscriptProps["onPinEvidence"];
}) {
  const blocks = props.entry.blocks.map(block => (
    <Block
      key={`${props.entry.id}:${block.id}`}
      colors={props.colors}
      entry={props.entry}
      block={block}
      availableWidth={props.availableWidth}
      interactive={props.interactive}
      onAction={props.onAction}
      onEvidence={props.onEvidence}
      onPinEvidence={props.onPinEvidence}
    />
  ));

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
        {blocks}
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
        {blocks}
      </box>
    </box>
  );
}

export function Transcript(props: TranscriptProps) {
  return (
    <box flexDirection="column" flexGrow={1} minHeight={0} onMouseDown={props.onFocus}>
      <scrollbox
        id="transcript-scrollbox"
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
        <Welcome
          colors={props.colors}
          compact={props.compact}
          artwork={props.artwork}
          availableWidth={props.availableWidth}
          workspaceLabel={props.workspaceLabel}
        />
        {props.entries.map(entry => (
          <Entry
            key={entry.id}
            colors={props.colors}
            entry={entry}
            availableWidth={props.availableWidth}
            interactive={props.interactive}
            onAction={props.onAction}
            onEvidence={props.onEvidence}
            onPinEvidence={props.onPinEvidence}
          />
        ))}
        {props.operationSceneVisible && props.context.operation.status === "running" ? (
          <OperationScene
            colors={props.colors}
            context={props.context}
            availableWidth={props.availableWidth}
            compact={props.compact}
          />
        ) : null}
      </scrollbox>
    </box>
  );
}
