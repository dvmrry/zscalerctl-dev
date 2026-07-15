import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useState,
  type ReactNode
} from "react";

import {
  DEFAULT_SPINNER,
  SPINNER_INTERVAL_MS,
  SPINNERS,
  spinnerFrame,
  type SpinnerType
} from "./spinner.ts";

const SpinnerFrameContext = createContext(spinnerFrame(DEFAULT_SPINNER, 0));

export function SpinnerFrameProvider(props: {
  readonly spinner: SpinnerType;
  readonly active: boolean;
  readonly children?: ReactNode;
}) {
  const [frameIndex, setFrameIndex] = useState(0);
  const frameCount = SPINNERS[props.spinner].frames.length;

  useEffect(() => {
    setFrameIndex(0);
    if (!props.active) return;
    const timer = setInterval(
      () => setFrameIndex(current => (current + 1) % frameCount),
      SPINNER_INTERVAL_MS
    );
    return () => clearInterval(timer);
  }, [frameCount, props.active, props.spinner]);

  return createElement(
    SpinnerFrameContext.Provider,
    {value: spinnerFrame(props.spinner, frameIndex)},
    props.children
  );
}

export function useSpinnerFrame(): string {
  return useContext(SpinnerFrameContext);
}
