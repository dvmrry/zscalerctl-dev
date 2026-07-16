import {TextAttributes} from "@opentui/core";
import type {ReactNode} from "react";

import type {Palette} from "../theme.ts";

export interface PaneProps {
  readonly colors: Palette;
  readonly title?: string;
  readonly shortcut?: string;
  readonly subtitle?: string;
  readonly active?: boolean;
  readonly collapsed?: boolean;
  readonly dense?: boolean;
  readonly onTitleClick?: () => void;
  readonly children?: ReactNode;
  readonly flexGrow?: number;
  readonly flexShrink?: number;
  readonly height?: number | "auto" | `${number}%`;
  readonly minHeight?: number;
}

export function Pane(props: PaneProps) {
  const borderColor = props.active === true ? props.colors.borderActive : props.colors.border;
  return (
    <box
      border={["left"]}
      borderColor={borderColor}
      backgroundColor={props.colors.panel}
      flexDirection="column"
      flexGrow={props.flexGrow}
      flexShrink={props.flexShrink}
      height={props.height}
      minHeight={props.minHeight}
      paddingTop={props.dense === true ? 0 : 1}
      paddingBottom={props.dense === true ? 0 : 1}
      paddingLeft={1}
      paddingRight={1}
    >
      {props.title === undefined ? null : (
        <box
          flexDirection="row"
          justifyContent="space-between"
          width="100%"
          marginBottom={props.collapsed === true || props.dense === true ? 0 : 1}
          onMouseDown={props.onTitleClick}
        >
          <text fg={props.colors.text} attributes={TextAttributes.BOLD}>
            {props.shortcut === undefined ? "" : `[${props.shortcut}] `}{props.title}
          </text>
          {props.subtitle === undefined ? null : (
            <text fg={props.colors.textMuted}>{props.subtitle}</text>
          )}
        </box>
      )}
      {props.collapsed === true ? null : props.children}
    </box>
  );
}
