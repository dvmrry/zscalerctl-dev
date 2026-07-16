import type {MouseEvent} from "@opentui/core";

import type {PinnedEvidence, TranscriptEvidence} from "../model.ts";
import {fitCellText, safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";

export interface WorkingSetProps {
  readonly colors: Palette;
  readonly current: TranscriptEvidence;
  readonly pins: readonly PinnedEvidence[];
  readonly availableWidth: number;
  readonly compact: boolean;
  readonly onPinCurrent: () => void;
  readonly onActivate: (pin: PinnedEvidence) => void;
  readonly onRemove: (id: number) => void;
}

function invoke(event: MouseEvent, action: () => void): void {
  event.stopPropagation();
  action();
}

export function WorkingSet(props: WorkingSetProps) {
  const width = Math.max(20, props.availableWidth);
  const currentActionWidth = 8;
  const currentText = fitCellText(
    safeInlineText(`${props.current.label} · ${props.current.kind} · ${props.current.preview}`, 240),
    Math.max(8, width - 12 - currentActionWidth)
  );
  const maximumVisiblePins = width < 52 ? 1 : width < 82 ? 2 : 3;
  const visiblePins = props.pins.slice(-maximumVisiblePins);
  const hiddenPins = Math.max(0, props.pins.length - visiblePins.length);
  const pinWidth = Math.max(8, Math.floor((width - 10 - (hiddenPins > 0 ? 4 : 0)) / Math.max(1, visiblePins.length)));

  return (
    <box
      flexDirection="column"
      flexShrink={0}
      border={["top"]}
      borderColor={props.colors.border}
      paddingLeft={1}
      paddingRight={1}
    >
      <box height={1} flexShrink={0} flexDirection="row" gap={1}>
        <text fg={props.colors.textMuted}>Context</text>
        <text fg={props.colors.text} flexGrow={1} wrapMode="none">{currentText}</text>
        <text
          id="working-set-pin-current"
          fg={props.colors.accent}
          onMouseDown={event => invoke(event, props.onPinCurrent)}
        >
          + pin · P
        </text>
      </box>
      {props.compact ? null : (
        <box height={1} flexShrink={0} flexDirection="row" gap={1}>
          <text fg={props.colors.textMuted}>Pinned</text>
          {visiblePins.length === 0 ? (
            <text fg={props.colors.textMuted}>none · saved evidence survives result changes</text>
          ) : visiblePins.map(pin => (
            <box key={pin.id} flexDirection="row" backgroundColor={props.colors.surface}>
              <text
                id={`working-set-pin-${pin.id}`}
                fg={props.colors.accentSecondary}
                onMouseDown={event => invoke(event, () => props.onActivate(pin))}
              >
                {fitCellText(pin.label, Math.max(4, pinWidth - 2))}
              </text>
              <text
                id={`working-set-remove-${pin.id}`}
                fg={props.colors.textMuted}
                onMouseDown={event => invoke(event, () => props.onRemove(pin.id))}
              > ×</text>
            </box>
          ))}
          {hiddenPins === 0 ? null : <text fg={props.colors.textMuted}>+{hiddenPins}</text>}
        </box>
      )}
    </box>
  );
}
