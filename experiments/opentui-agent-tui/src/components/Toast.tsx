import {TextAttributes} from "@opentui/core";

import {OVERLAY_Z_INDEX} from "../overlay.ts";
import {fitCellText, type SafeString} from "../text.ts";
import type {Palette} from "../theme.ts";
import {toastColorRole, toastMarker, type ToastTone} from "../toast.ts";

export interface ToastProps {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly message: SafeString;
  readonly tone: ToastTone;
}

export function Toast(props: ToastProps) {
  const width = Math.max(1, Math.min(52, props.viewportWidth - 2));
  const color = props.colors[toastColorRole(props.tone)];
  return (
    <box
      position="absolute"
      top={1}
      right={1}
      width={width}
      height={3}
      zIndex={OVERLAY_Z_INDEX.toast}
      flexDirection="row"
      alignItems="center"
      gap={1}
      paddingLeft={1}
      paddingRight={1}
      backgroundColor={props.colors.panelRaised}
      border={['left']}
      borderColor={color}
    >
      <text fg={color} attributes={TextAttributes.BOLD}>{toastMarker(props.tone)}</text>
      <text fg={props.colors.text} wrapMode="none">{fitCellText(props.message, Math.max(1, width - 5))}</text>
    </box>
  );
}
