import {TextAttributes, type BoxRenderable} from "@opentui/core";
import type {Ref} from "react";

import {
  poisonBannerForWidth,
  poisonBeamSegments,
  type PoisonBanner
} from "../brand.ts";
import type {Palette} from "../theme.ts";
import {useMotionFrame} from "../useMotion.ts";

function PoisonArtwork(props: {
  readonly banner: PoisonBanner;
  readonly colors: Palette;
  readonly bannerRef?: Ref<BoxRenderable>;
}) {
  const motion = useMotionFrame();
  return (
    <box flexDirection="column" marginBottom={1}>
      <box id="welcome-poison-banner" ref={props.bannerRef} flexDirection="column">
        {props.banner.lines.map((line, index) => {
          const segments = poisonBeamSegments(
            line,
            motion.frameIndex,
            props.banner.width,
            motion.active && motion.mode === "full"
          );
          const baseColor = index < 3
            ? props.colors.accent
            : index < 7 ? props.colors.text : props.colors.accentSecondary;
          return (
            <text key={index} wrapMode="none">
              <span style={{fg: baseColor}}>{segments.before}</span>
              <span style={{fg: props.colors.warning}}>{segments.beam}</span>
              <span style={{fg: baseColor}}>{segments.after}</span>
            </text>
          );
        })}
      </box>
      <text fg={props.colors.textMuted}>OpenTUI lab</text>
    </box>
  );
}

export function Welcome(props: {
  readonly colors: Palette;
  readonly compact: boolean;
  readonly artwork: boolean;
  readonly availableWidth: number;
  readonly workspaceLabel: string;
  readonly poisonBannerRef?: Ref<BoxRenderable>;
}) {
  if (props.compact) {
    return (
      <box flexDirection="column" marginBottom={1} paddingLeft={1}>
        <text fg={props.colors.text} attributes={TextAttributes.BOLD}>
          <span style={{fg: props.colors.accent}}>◆ </span>zscalerctl <span style={{fg: props.colors.textMuted}}>OpenTUI lab</span>
        </text>
        <text fg={props.colors.textMuted}><span style={{fg: props.colors.success}}>●</span> {props.workspaceLabel} · tenant read-only · no model</text>
      </box>
    );
  }
  const banner = props.artwork ? poisonBannerForWidth(props.availableWidth) : undefined;
  return (
    <box
      flexDirection="column"
      paddingLeft={2}
      paddingTop={1}
      paddingBottom={1}
      marginBottom={1}
    >
      {banner === undefined ? (
        <text fg={props.colors.text} attributes={TextAttributes.BOLD}>
          <span style={{fg: props.colors.accent}}>◆ </span>zscalerctl <span style={{fg: props.colors.textMuted}}>OpenTUI lab</span>
        </text>
      ) : (
        <PoisonArtwork banner={banner} colors={props.colors} bannerRef={props.poisonBannerRef} />
      )}
      <text fg={props.colors.textMuted}>Agentic tenant explorer with a structured data workspace.</text>
      <box flexDirection="row" gap={2}>
        <text fg={props.colors.success}>● tenant read-only</text>
        <text fg={props.colors.textMuted}>{props.workspaceLabel}</text>
        <text fg={props.colors.warning}>no model attached</text>
      </box>
      <text fg={props.colors.textMuted}>
        {props.workspaceLabel === "fixture"
          ? "/demo reloads data · /find searches · Ctrl+O opens the inspector"
          : "/catalog browses resources · /find searches data · Ctrl+O inspects"}
      </text>
    </box>
  );
}
