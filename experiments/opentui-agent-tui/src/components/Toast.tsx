import {TextAttributes} from "@opentui/core";

import {OVERLAY_Z_INDEX} from "../overlay.ts";
import {safeInlineText} from "../text.ts";
import type {Palette} from "../theme.ts";

export interface ToastProps {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly message: string;
  readonly tone: "success" | "warning";
}

export function Toast(props: ToastProps) {
  const width = Math.max(1, Math.min(52, props.viewportWidth - 2));
  const color = props.tone === "success" ? props.colors.success : props.colors.warning;
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
      <text fg={color} attributes={TextAttributes.BOLD}>{props.tone === "success" ? "✓" : "!"}</text>
      <text fg={props.colors.text} wrapMode="none">{safeInlineText(props.message, Math.max(1, width - 5))}</text>
    </box>
  );
}
