import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode
} from "react";

import {
  DEFAULT_MOTION_MODE,
  motionInterval,
  SYSTEM_MOTION_TIMERS,
  type MotionTimerDriver,
  type MotionMode
} from "./motion.ts";
import {
  DEFAULT_SPINNER,
  spinnerFrame,
  type SpinnerType
} from "./spinner.ts";

const MAXIMUM_FRAME_INDEX = 1_000_000;

export interface MotionFrameState {
  readonly active: boolean;
  readonly frameIndex: number;
  readonly mode: MotionMode;
  readonly spinner: SpinnerType;
}

const MotionFrameContext = createContext<MotionFrameState>({
  active: false,
  frameIndex: 0,
  mode: DEFAULT_MOTION_MODE,
  spinner: DEFAULT_SPINNER
});

export function MotionProvider(props: {
  readonly spinner: SpinnerType;
  readonly mode: MotionMode;
  readonly active: boolean;
  readonly timers?: MotionTimerDriver;
  readonly children?: ReactNode;
}) {
  const [frameIndex, setFrameIndex] = useState(0);

  useEffect(() => {
    setFrameIndex(0);
    const interval = motionInterval(props.mode);
    if (!props.active || interval === undefined) return;
    let acceptingFrames = true;
    const timers = props.timers ?? SYSTEM_MOTION_TIMERS;
    const timer = timers.setInterval(
      () => {
        if (!acceptingFrames) return;
        setFrameIndex(current => (current + 1) % MAXIMUM_FRAME_INDEX);
      },
      interval
    );
    return () => {
      acceptingFrames = false;
      timers.clearInterval(timer);
    };
  }, [props.active, props.mode, props.spinner, props.timers]);

  const value = useMemo<MotionFrameState>(() => ({
    active: props.active && props.mode !== "off",
    frameIndex,
    mode: props.mode,
    spinner: props.spinner
  }), [frameIndex, props.active, props.mode, props.spinner]);

  return createElement(MotionFrameContext.Provider, {value}, props.children);
}

export function useMotionFrame(): MotionFrameState {
  return useContext(MotionFrameContext);
}

export function useSpinnerFrame(): string {
  const motion = useMotionFrame();
  return spinnerFrame(motion.spinner, motion.frameIndex);
}
