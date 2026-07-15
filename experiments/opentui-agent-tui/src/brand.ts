export interface PoisonBanner {
  readonly label: string;
  readonly lines: readonly string[];
  readonly width: number;
}

// Generated once with FIGlet 2.2.5's Poison font. Keeping the rendered cells
// in source avoids loading a font parser and the complete font catalog at run
// time. Attribution lives in THIRD_PARTY_NOTICES.md.
export const POISON_ZSCALERCTL: PoisonBanner = Object.freeze({
  label: "zscalerctl",
  width: 97,
  lines: Object.freeze([
    "@@@@@@@@   @@@@@@    @@@@@@@   @@@@@@   @@@       @@@@@@@@  @@@@@@@    @@@@@@@  @@@@@@@  @@@",
    "@@@@@@@@  @@@@@@@   @@@@@@@@  @@@@@@@@  @@@       @@@@@@@@  @@@@@@@@  @@@@@@@@  @@@@@@@  @@@",
    "     @@!  !@@       !@@       @@!  @@@  @@!       @@!       @@!  @@@  !@@         @@!    @@!",
    "    !@!   !@!       !@!       !@!  @!@  !@!       !@!       !@!  @!@  !@!         !@!    !@!",
    "   @!!    !!@@!!    !@!       @!@!@!@!  @!!       @!!!:!    @!@!!@!   !@!         @!!    @!!",
    "  !!!      !!@!!!   !!!       !!!@!!!!  !!!       !!!!!:    !!@!@!    !!!         !!!    !!!",
    " !!:           !:!  :!!       !!:  !!!  !!:       !!:       !!: :!!   :!!         !!:    !!:",
    ":!:           !:!   :!:       :!:  !:!   :!:      :!:       :!:  !:!  :!:         :!:     :!:",
    " :: ::::  :::: ::    ::: :::  ::   :::   :: ::::   :: ::::  ::   :::   ::: :::     ::     :: ::::",
    ": :: : :  :: : :     :: :: :   :   : :  : :: : :  : :: ::    :   : :   :: :: :     :     : :: : :"
  ])
});

export function poisonBannerForWidth(availableWidth: number): PoisonBanner | undefined {
  const width = Math.max(0, Math.floor(availableWidth));
  if (width >= POISON_ZSCALERCTL.width) return POISON_ZSCALERCTL;
  return undefined;
}

export interface BeamSegments {
  readonly before: string;
  readonly beam: string;
  readonly after: string;
}

export function poisonBeamSegments(
  line: string,
  frameIndex: number,
  bannerWidth: number,
  active: boolean
): BeamSegments {
  if (!active || line.length === 0 || bannerWidth <= 0) {
    return {before: line, beam: "", after: ""};
  }
  const normalizedFrame = Number.isSafeInteger(frameIndex) && frameIndex >= 0 ? frameIndex : 0;
  const start = (normalizedFrame * 7) % bannerWidth;
  if (start >= line.length) return {before: line, beam: "", after: ""};
  const end = Math.min(line.length, start + 7);
  return {
    before: line.slice(0, start),
    beam: line.slice(start, end),
    after: line.slice(end)
  };
}
