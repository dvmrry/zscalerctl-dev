import {TextAttributes} from "@opentui/core";

import type {ContextState} from "../model.ts";
import type {MotionMode} from "../motion.ts";
import {fitCellText, safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {useMotionFrame} from "../useMotion.ts";
import {normalizeOperationProgress} from "./OperationIndicator.tsx";

export function operationTrack(width: number, frameIndex: number, mode: MotionMode): string {
  const cells = Math.max(1, Math.floor(width));
  const normalizedFrame = Number.isSafeInteger(frameIndex) && frameIndex >= 0 ? frameIndex : 0;
  let position = Math.floor((cells - 1) / 2);
  if (mode === "full" && cells > 1) {
    const cycle = (cells - 1) * 2;
    const step = normalizedFrame % cycle;
    position = step < cells ? step : cycle - step;
  }
  return `${"━".repeat(position)}◆${"━".repeat(cells - position - 1)}`;
}

function progressCounter(context: ContextState): string | undefined {
  const progress = normalizeOperationProgress(
    context.operation.completed,
    context.operation.total
  );
  return progress === undefined ? undefined : `${progress.completed}/${progress.total}`;
}

export function OperationScene(props: {
  readonly colors: Palette;
  readonly context: ContextState;
  readonly availableWidth: number;
  readonly compact: boolean;
  readonly motionFrame?: number;
}) {
  const motion = useMotionFrame();
  const frameIndex = props.motionFrame ?? motion.frameIndex;
  const mode = motion.mode;
  const width = Math.max(12, Math.floor(props.availableWidth));
  const label = safeInlineText(props.context.operation.label, 240);
  const counter = progressCounter(props.context);

  if (props.compact || width < 52) {
    const counterWidth = counter === undefined ? 0 : Bun.stringWidth(counter) + 1;
    const activity = operationTrack(mode === "off" ? 3 : 5, frameIndex, mode);
    const prefix = `${activity} `;
    return (
      <box flexDirection="row" flexShrink={0} marginTop={1} paddingLeft={1} justifyContent="space-between">
        <text fg={props.colors.accent} wrapMode="none">
          {prefix}<span style={{fg: props.colors.text}}>{fitCellText(label, Math.max(4, width - Bun.stringWidth(prefix) - counterWidth - 1))}</span>
        </text>
        {counter === undefined ? null : <text fg={props.colors.textMuted}>{counter}</text>}
      </box>
    );
  }

  const innerWidth = Math.max(16, width - 3);
  const transport = fitCellText(safeInlineText(props.context.transport, 160), Math.max(8, Math.floor(innerWidth * 0.34)));
  const destination = `[ ${transport} ]`;
  const routeSuffix = `▶ ${destination}`;
  const trackWidth = Math.max(6, Math.min(30, innerWidth - Bun.stringWidth(routeSuffix) - Bun.stringWidth("local ") - 1));
  const authority = fitCellText(safeInlineText(props.context.authority, 120), Math.max(8, innerWidth - 20));
  const counterWidth = counter === undefined ? 0 : Bun.stringWidth(counter) + 1;

  return (
    <box
      flexDirection="column"
      flexShrink={0}
      border={["left"]}
      borderColor={props.colors.accent}
      backgroundColor={props.colors.panel}
      paddingTop={1}
      paddingBottom={1}
      paddingLeft={2}
      paddingRight={1}
      marginTop={1}
    >
      <text fg={props.colors.accent} attributes={TextAttributes.BOLD} wrapMode="none">
        ACTIVE OPERATION <span style={{fg: props.colors.textMuted}}>· {authority}</span>
      </text>
      <text wrapMode="none">
        <span style={{fg: props.colors.textMuted}}>local </span>
        <span style={{fg: props.colors.accent}}>{operationTrack(trackWidth, frameIndex, mode)}</span>
        <span style={{fg: props.colors.textMuted}}>{routeSuffix}</span>
      </text>
      <box flexDirection="row" justifyContent="space-between">
        <text fg={props.colors.text} wrapMode="none">
          {fitCellText(label, Math.max(6, innerWidth - counterWidth))}
        </text>
        {counter === undefined ? null : <text fg={props.colors.textMuted}>{counter}</text>}
      </box>
    </box>
  );
}
