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

function fixedCellText(value: string, width: number, align: "left" | "right" = "left"): string {
  const cells = Math.max(1, Math.floor(width));
  const fitted = fitCellText(value, cells);
  const padding = " ".repeat(Math.max(0, cells - Bun.stringWidth(fitted)));
  return align === "right" ? `${padding}${fitted}` : `${fitted}${padding}`;
}

function fitProgressCounter(counter: string, width: number): string {
  const cells = Math.max(1, Math.floor(width));
  if (Bun.stringWidth(counter) <= cells) return counter;
  if (cells < 3) return fitCellText(counter, cells);
  const separator = counter.indexOf("/");
  if (separator < 0) return fitCellText(counter, cells);
  const numericCells = cells - 1;
  const completedCells = Math.floor(numericCells / 2);
  const totalCells = numericCells - completedCells;
  return `${fitCellText(counter.slice(0, separator), completedCells)}/${fitCellText(counter.slice(separator + 1), totalCells)}`;
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
    const activity = operationTrack(mode === "off" ? 3 : 5, frameIndex, mode);
    const prefix = `${activity} `;
    const contentWidth = Math.max(1, width - 1);
    const detailWidth = Math.max(1, contentWidth - Bun.stringWidth(prefix));
    const counterWidth = counter === undefined
      ? 0
      : Math.max(1, Math.min(
        detailWidth - 2,
        Bun.stringWidth(counter),
        Math.max(3, Math.floor(detailWidth / 2))
      ));
    const labelWidth = counter === undefined
      ? detailWidth
      : Math.max(1, detailWidth - counterWidth - 1);
    const labelText = fixedCellText(label, labelWidth);
    const counterText = counter === undefined
      ? ""
      : ` ${fixedCellText(fitProgressCounter(counter, counterWidth), counterWidth, "right")}`;
    return (
      <box id="operation-scene-compact" flexShrink={0} marginTop={1} paddingLeft={1}>
        <text id="operation-scene-compact-text" fg={props.colors.accent} wrapMode="none">
          {prefix}<span style={{fg: props.colors.text}}>{labelText}</span><span style={{fg: props.colors.textMuted}}>{counterText}</span>
        </text>
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
