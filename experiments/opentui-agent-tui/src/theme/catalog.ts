import aura from "./assets/aura.json" with {type: "json"};
import ayu from "./assets/ayu.json" with {type: "json"};
import carbonfox from "./assets/carbonfox.json" with {type: "json"};
import catppuccinFrappe from "./assets/catppuccin-frappe.json" with {type: "json"};
import catppuccinMacchiato from "./assets/catppuccin-macchiato.json" with {type: "json"};
import catppuccin from "./assets/catppuccin.json" with {type: "json"};
import cobalt2 from "./assets/cobalt2.json" with {type: "json"};
import cursor from "./assets/cursor.json" with {type: "json"};
import dracula from "./assets/dracula.json" with {type: "json"};
import everforest from "./assets/everforest.json" with {type: "json"};
import flexoki from "./assets/flexoki.json" with {type: "json"};
import github from "./assets/github.json" with {type: "json"};
import gruvbox from "./assets/gruvbox.json" with {type: "json"};
import kanagawa from "./assets/kanagawa.json" with {type: "json"};
import lucentOrng from "./assets/lucent-orng.json" with {type: "json"};
import material from "./assets/material.json" with {type: "json"};
import matrix from "./assets/matrix.json" with {type: "json"};
import mercury from "./assets/mercury.json" with {type: "json"};
import monokai from "./assets/monokai.json" with {type: "json"};
import nightowl from "./assets/nightowl.json" with {type: "json"};
import nord from "./assets/nord.json" with {type: "json"};
import oneDark from "./assets/one-dark.json" with {type: "json"};
import opencode from "./assets/opencode.json" with {type: "json"};
import orng from "./assets/orng.json" with {type: "json"};
import osakaJade from "./assets/osaka-jade.json" with {type: "json"};
import palenight from "./assets/palenight.json" with {type: "json"};
import rosepine from "./assets/rosepine.json" with {type: "json"};
import solarized from "./assets/solarized.json" with {type: "json"};
import synthwave84 from "./assets/synthwave84.json" with {type: "json"};
import tokyonight from "./assets/tokyonight.json" with {type: "json"};
import vercel from "./assets/vercel.json" with {type: "json"};
import vesper from "./assets/vesper.json" with {type: "json"};
import zenburn from "./assets/zenburn.json" with {type: "json"};
import type {ThemeDefinition} from "./engine.ts";

export const THEME_NAMES = [
  "opencode",
  "aura",
  "ayu",
  "carbonfox",
  "catppuccin",
  "catppuccin-frappe",
  "catppuccin-macchiato",
  "cobalt2",
  "cursor",
  "dracula",
  "everforest",
  "flexoki",
  "github",
  "gruvbox",
  "kanagawa",
  "lucent-orng",
  "material",
  "matrix",
  "mercury",
  "monokai",
  "nightowl",
  "nord",
  "one-dark",
  "orng",
  "osaka-jade",
  "palenight",
  "rosepine",
  "solarized",
  "synthwave84",
  "tokyonight",
  "vercel",
  "vesper",
  "zenburn",
  "signal",
  "tron",
  "cyberpunk",
  "mono"
] as const;

export type ThemeName = (typeof THEME_NAMES)[number];

const localThemes = {
  signal: {
    theme: {
      primary: "#38bdf8",
      secondary: "#a78bfa",
      accent: "primary",
      error: "#fb7185",
      warning: "#fbbf24",
      success: "#3ddc97",
      info: "primary",
      text: "#e6edf3",
      textMuted: "#8b98a8",
      selectedListItemText: "text",
      background: "#0d1117",
      backgroundPanel: "#151b23",
      backgroundElement: "#1b2430",
      backgroundMenu: "#1e3a5f",
      border: "#334155",
      borderActive: "primary",
      borderSubtle: "#202a38"
    }
  },
  tron: {
    theme: {
      primary: "#22d3ee",
      secondary: "#f97316",
      accent: "primary",
      error: "#fb7185",
      warning: "#fbbf24",
      success: "#5eead4",
      info: "primary",
      text: "#d8fbff",
      textMuted: "#6f9eaa",
      selectedListItemText: "text",
      background: "#030b12",
      backgroundPanel: "#071823",
      backgroundElement: "#0a2230",
      backgroundMenu: "#0e4053",
      border: "#155e75",
      borderActive: "primary",
      borderSubtle: "#0d3443"
    }
  },
  cyberpunk: {
    theme: {
      primary: "#f0e70a",
      secondary: "#ff2a6d",
      accent: "primary",
      error: "#ff2a6d",
      warning: "#f0e70a",
      success: "#05d9e8",
      info: "#05d9e8",
      text: "#f7f1ff",
      textMuted: "#a786b0",
      selectedListItemText: "text",
      background: "#100718",
      backgroundPanel: "#1a0d26",
      backgroundElement: "#251133",
      backgroundMenu: "#4a174f",
      border: "#6b2d72",
      borderActive: "primary",
      borderSubtle: "#3f1b4a"
    }
  },
  mono: {
    theme: {
      primary: "#e5e5e5",
      secondary: "#bdbdbd",
      accent: "primary",
      error: "#ffffff",
      warning: "#bbbbbb",
      success: "#d4d4d4",
      info: "primary",
      text: "#f0f0f0",
      textMuted: "#999999",
      selectedListItemText: "text",
      background: "#101010",
      backgroundPanel: "#191919",
      backgroundElement: "#222222",
      backgroundMenu: "#3a3a3a",
      border: "#555555",
      borderActive: "primary",
      borderSubtle: "#333333"
    }
  }
} as const satisfies Readonly<Record<"signal" | "tron" | "cyberpunk" | "mono", ThemeDefinition>>;

export const THEME_CATALOG = {
  opencode,
  aura,
  ayu,
  carbonfox,
  catppuccin,
  "catppuccin-frappe": catppuccinFrappe,
  "catppuccin-macchiato": catppuccinMacchiato,
  cobalt2,
  cursor,
  dracula,
  everforest,
  flexoki,
  github,
  gruvbox,
  kanagawa,
  "lucent-orng": lucentOrng,
  material,
  matrix,
  mercury,
  monokai,
  nightowl,
  nord,
  "one-dark": oneDark,
  orng,
  "osaka-jade": osakaJade,
  palenight,
  rosepine,
  solarized,
  synthwave84,
  tokyonight,
  vercel,
  vesper,
  zenburn,
  ...localThemes
} satisfies Readonly<Record<ThemeName, ThemeDefinition>>;
