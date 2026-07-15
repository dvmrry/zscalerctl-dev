import type {OperationState} from "../model.ts";
import {fitCellText, safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";
import {useSpinnerFrame} from "../useSpinnerFrame.ts";

export interface NormalizedProgress {
  readonly completed: number;
  readonly total: number;
  readonly ratio: number;
}

export interface ProgressBarSegments {
  readonly complete: string;
  readonly head: string;
  readonly remaining: string;
}

export function normalizeOperationProgress(completed?: number, total?: number): NormalizedProgress | undefined {
  if (!Number.isSafeInteger(completed) || !Number.isSafeInteger(total)) return undefined;
  if (completed === undefined || total === undefined || completed < 0 || total <= 0 || completed > total) return undefined;
  return {completed, total, ratio: completed / total};
}

export function progressBarSegments(progress: NormalizedProgress, width: number): ProgressBarSegments {
  const cells = Math.max(0, Math.floor(width));
  if (cells === 0) return {complete: "", head: "", remaining: ""};
  if (progress.completed === progress.total) {
    return {complete: "━".repeat(cells), head: "", remaining: ""};
  }
  const occupied = progress.completed === 0 ? 0 : Math.ceil(progress.ratio * cells);
  const complete = progress.completed === 0 ? 0 : Math.max(0, occupied - 1);
  const head = progress.completed === 0 ? "" : "╺";
  return {
    complete: "━".repeat(complete),
    head,
    remaining: "·".repeat(Math.max(0, cells - complete - (head.length === 0 ? 0 : 1)))
  };
}

function operationColor(operation: OperationState, colors: Palette): string {
  switch (operation.status) {
    case "error": return colors.danger;
    case "running": return colors.accent;
    case "complete": return colors.success;
    case "idle": return colors.textMuted;
  }
}

function ProgressBar(props: {
  readonly colors: Palette;
  readonly progress: NormalizedProgress;
  readonly width: number;
}) {
  const segments = progressBarSegments(props.progress, props.width);
  return (
    <text wrapMode="none">
      <span style={{fg: props.colors.accent}}>{segments.complete}{segments.head}</span>
      <span style={{fg: props.colors.border}}>{segments.remaining}</span>
    </text>
  );
}

export function OperationIndicator(props: {
  readonly colors: Palette;
  readonly operation: OperationState;
  readonly detail: string;
  readonly availableWidth: number;
  readonly activityFrame?: string;
}) {
  const sharedActivityFrame = useSpinnerFrame();
  const activityFrame = props.activityFrame ?? sharedActivityFrame;
  const width = Math.max(1, Math.floor(props.availableWidth));
  const progress = props.operation.status === "running"
    ? normalizeOperationProgress(props.operation.completed, props.operation.total)
    : undefined;
  const counter = progress === undefined ? undefined : `${progress.completed}/${progress.total}`;
  const activity = props.operation.status === "running" ? `${activityFrame} ` : "";
  const rawLabel = safeInlineText(`${activity}${props.operation.label}`, 240);
  const color = operationColor(props.operation, props.colors);

  if (progress === undefined || counter === undefined) {
    if (width < 8) {
      const compactLabel = props.operation.status === "running"
        ? width === 1 ? "·" : fitCellText(activityFrame, width)
        : fitCellText(rawLabel, width);
      return (
        <box height={1} flexShrink={0} marginTop={1}>
          <text fg={color} wrapMode="none">{compactLabel}</text>
        </box>
      );
    }
    const detail = fitCellText(safeInlineText(props.detail, 120), Math.max(1, Math.floor(width * 0.4)));
    const label = fitCellText(rawLabel, Math.max(1, width - Bun.stringWidth(detail) - 1));
    return (
      <box height={1} flexShrink={0} marginTop={1} flexDirection="row" justifyContent="space-between">
        <text fg={color} wrapMode="none">{label}</text>
        <text fg={props.colors.textMuted} wrapMode="none">{detail}</text>
      </box>
    );
  }

  const counterWidth = Bun.stringWidth(counter);
  if (width >= counterWidth + 17) {
    const barWidth = Math.max(4, Math.min(10, width - counterWidth - 10));
    const rightWidth = barWidth + counterWidth + 1;
    const label = fitCellText(rawLabel, Math.max(1, width - rightWidth - 1));
    return (
      <box height={1} flexShrink={0} marginTop={1} flexDirection="row" justifyContent="space-between">
        <text fg={color} wrapMode="none">{label}</text>
        <box flexDirection="row" gap={1}>
          <ProgressBar colors={props.colors} progress={progress} width={barWidth} />
          <text fg={props.colors.textMuted} wrapMode="none">{counter}</text>
        </box>
      </box>
    );
  }

  if (width >= counterWidth + 5) {
    const label = fitCellText(rawLabel, Math.max(1, width - counterWidth - 1));
    return (
      <box height={1} flexShrink={0} marginTop={1} flexDirection="row" justifyContent="space-between">
        <text fg={color} wrapMode="none">{label}</text>
        <text fg={props.colors.textMuted} wrapMode="none">{counter}</text>
      </box>
    );
  }

  if (width >= counterWidth + 3) {
    return (
      <box height={1} flexShrink={0} marginTop={1} flexDirection="row" justifyContent="space-between">
        <text fg={color} wrapMode="none">{fitCellText(activityFrame, 2)}</text>
        <text fg={props.colors.textMuted} wrapMode="none">{counter}</text>
      </box>
    );
  }

  return (
    <box height={1} flexShrink={0} marginTop={1}>
      <text fg={color} wrapMode="none">{width === 1 ? "·" : fitCellText(activityFrame, width)}</text>
    </box>
  );
}
