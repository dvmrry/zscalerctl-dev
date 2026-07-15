# Third-party notices

## FIGlet Poison banner

The static `zscalerctl` banner in `src/brand.ts` was generated with FIGlet
2.2.5 using its contributed Poison font:

```sh
figlet -w 100 -f poison zscalerctl
```

The blank outer rows and right-side padding were removed before embedding the
result. The Poison font header credits Vinney Thai for the FIGlet composition,
numbers, and punctuation; David Issel for the uppercase characters; and dates
the font October 23, 1994. The source font is distributed with FIGlet and is
also available in the pinned FIGlet.js font catalog:

<https://github.com/patorjk/figlet.js/blob/v1.11.0/fonts/Poison.flf>

No FIGlet parser or font file is shipped or loaded at runtime.

## OpenCode references

The JSON files under `src/theme/assets/` are adapted verbatim from the OpenCode
theme catalog at commit `05c3e40a4e641732b991499000ca479e5dad4b02`:

<https://github.com/anomalyco/opencode/tree/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/theme/assets>

The resolver in `src/theme/engine.ts` is an independent TypeScript adaptation
of OpenCode's theme-resolution semantics for this React/OpenTUI experiment.

The overlay, picker, command-routing, and transient-feedback interaction models
were independently implemented after studying OpenCode's dialog provider,
select dialog, command palette, session list/timeline, keymap, toast, and prompt
autocomplete at the same commit:

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/ui/dialog.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/ui/dialog-select.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/component/command-palette.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/component/dialog-session-list.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/routes/session/dialog-timeline.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/keymap.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/ui/toast.tsx>

<https://github.com/anomalyco/opencode/blob/05c3e40a4e641732b991499000ca479e5dad4b02/packages/tui/src/component/prompt/autocomplete.tsx>

OpenCode is distributed under the following license:

> MIT License
>
> Copyright (c) 2025 opencode
>
> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all
> copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.
