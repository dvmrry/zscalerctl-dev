import {RGBA, type MouseEvent} from "@opentui/core";
import type {ReactNode} from "react";

import {
  OVERLAY_Z_INDEX,
  placeFloatingWindow,
  type FloatingWindowPlacement,
  type OverlayLayer
} from "../overlay.ts";
import type {Palette} from "../theme.ts";

type Percent = `${number}%`;
type Dimension = number | "auto" | Percent;

export interface FloatingWindowProps {
  readonly colors: Palette;
  readonly viewportWidth: number;
  readonly viewportHeight: number;
  readonly preferredWidth: number;
  readonly preferredHeight: number;
  readonly placement?: FloatingWindowPlacement;
  readonly layer?: OverlayLayer;
  readonly title?: string;
  readonly bottomTitle?: string;
  readonly onFocus?: () => void;
  readonly children?: ReactNode;
}

export function FloatingWindow(props: FloatingWindowProps) {
  const rect = placeFloatingWindow({
    viewportWidth: props.viewportWidth,
    viewportHeight: props.viewportHeight,
    preferredWidth: props.preferredWidth,
    preferredHeight: props.preferredHeight,
    placement: props.placement
  });

  return (
    <box
      position="absolute"
      left={rect.left}
      top={rect.top}
      width={rect.width}
      height={rect.height}
      zIndex={OVERLAY_Z_INDEX[props.layer ?? "dialog"]}
      flexDirection="column"
      backgroundColor={props.colors.panelRaised}
      border
      borderStyle="rounded"
      borderColor={props.colors.borderActive}
      title={props.title}
      titleColor={props.colors.text}
      bottomTitle={props.bottomTitle}
      onMouseDown={event => {
        event.stopPropagation();
        props.onFocus?.();
      }}
    >
      {props.children}
    </box>
  );
}

export interface OverlayBackdropProps {
  readonly layer?: OverlayLayer;
  readonly align?: "start" | "center" | "end";
  readonly dim?: number;
  readonly contentWidth: Dimension;
  readonly contentHeight: Dimension;
  readonly onDismiss: () => void;
  readonly children?: ReactNode;
}

export function OverlayBackdrop(props: OverlayBackdropProps) {
  const alignItems = props.align === "start" ? "flex-start" : props.align === "center" ? "center" : "flex-end";
  const keepOpen = (event: MouseEvent) => event.stopPropagation();
  return (
    <box
      position="absolute"
      top={0}
      left={0}
      right={0}
      bottom={0}
      zIndex={OVERLAY_Z_INDEX[props.layer ?? "dialog"]}
      alignItems={alignItems}
      backgroundColor={RGBA.fromInts(0, 0, 0, props.dim ?? 96)}
      onMouseDown={props.onDismiss}
    >
      <box
        width={props.contentWidth}
        height={props.contentHeight}
        onMouseDown={keepOpen}
      >
        {props.children}
      </box>
    </box>
  );
}
