import {TextAttributes, type KeyBinding, type KeyEvent, type TextareaRenderable} from "@opentui/core";
import {useEffect, useMemo, useRef, useState} from "react";

import {suggestionsFor, type CommandDescriptor, type FocusTarget} from "../model.ts";
import type {Palette} from "../theme.ts";
import {useSpinnerFrame} from "../useSpinnerFrame.ts";

const KEY_BINDINGS: readonly KeyBinding[] = [
  {name: "return", action: "submit"},
  {name: "kpenter", action: "submit"},
  {name: "return", shift: true, action: "newline"},
  {name: "kpenter", shift: true, action: "newline"}
];

export interface ComposerProps {
  readonly colors: Palette;
  readonly focus: FocusTarget;
  readonly busy: boolean;
  readonly commands: readonly CommandDescriptor[];
  readonly workspaceLabel: string;
  readonly availableWidth: number;
  readonly roomy: boolean;
  readonly activityFrame?: string;
  readonly onFocus: () => void;
  readonly onFocusNext: () => void;
  readonly onSubmit: (value: string) => void;
}

export function Composer(props: ComposerProps) {
  const sharedActivityFrame = useSpinnerFrame();
  const editor = useRef<TextareaRenderable | null>(null);
  const [draft, setDraft] = useState("");
  const [selected, setSelected] = useState(0);
  const [menuSuppressed, setMenuSuppressed] = useState(false);
  const compactChrome = props.availableWidth < 88;
  const minimalChrome = props.availableWidth < 58;
  const placeholder = minimalChrome
    ? "Ask…  / commands"
    : "Ask about tenant configuration…  / opens commands";
  const suggestions = useMemo(
    () => menuSuppressed ? [] : suggestionsFor(draft, props.commands),
    [draft, menuSuppressed, props.commands]
  );

  useEffect(() => {
    setSelected(index => Math.min(index, Math.max(0, suggestions.length - 1)));
  }, [suggestions.length]);

  const refreshDraft = () => {
    const value = editor.current?.plainText ?? "";
    setDraft(value);
    setMenuSuppressed(false);
  };

  const acceptSuggestion = (
    candidates: readonly (typeof suggestions)[number][] = suggestions,
    requestedIndex = selected
  ) => {
    const suggestion = candidates[Math.min(requestedIndex, Math.max(0, candidates.length - 1))];
    if (suggestion === undefined) return;
    const value = `${suggestion.command} `;
    editor.current?.setText(value);
    editor.current?.gotoBufferEnd();
    setDraft(value);
    setMenuSuppressed(true);
  };

  const handleKey = (event: KeyEvent) => {
    const liveDraft = editor.current?.plainText ?? draft;
    const liveSuggestions = menuSuppressed ? [] : suggestionsFor(liveDraft, props.commands);
    if (event.name === "tab" && !event.shift) {
      event.preventDefault();
      event.stopPropagation();
      if (liveSuggestions.length > 0) acceptSuggestion(liveSuggestions);
      else if (menuSuppressed && liveDraft.trimStart().startsWith("/")) setMenuSuppressed(false);
      else props.onFocusNext();
      return;
    }
    if (liveSuggestions.length === 0) return;
    if (event.name === "up" || event.name === "down") {
      event.preventDefault();
      event.stopPropagation();
      setSelected(index => {
        const delta = event.name === "up" ? -1 : 1;
        return (index + delta + liveSuggestions.length) % liveSuggestions.length;
      });
      return;
    }
    if (event.name === "escape") {
      event.preventDefault();
      event.stopPropagation();
      setMenuSuppressed(true);
      return;
    }
    if ((event.name === "return" || event.name === "kpenter") && liveDraft.trim() !== liveSuggestions[selected]?.command) {
      event.preventDefault();
      event.stopPropagation();
      acceptSuggestion(liveSuggestions);
    }
  };

  const submit = () => {
    const value = (editor.current?.plainText ?? "").trim();
    const busyControl = /^\/(?:cancel|quit)$/iu.test(value);
    if (value.length === 0 || (props.busy && !busyControl)) return;
    editor.current?.clear();
    setDraft("");
    setMenuSuppressed(false);
    props.onSubmit(value);
  };

  return (
    <box flexDirection="column" flexShrink={0} onMouseDown={props.onFocus}>
      {suggestions.length === 0 ? null : (
        <box
          flexDirection="column"
          backgroundColor={props.colors.panelRaised}
          border={["left"]}
          borderColor={props.colors.accent}
          paddingLeft={2}
          paddingRight={2}
          marginBottom={1}
        >
          {suggestions.map((item, index) => (
            <box
              key={item.command}
              flexDirection="row"
              gap={1}
              backgroundColor={index === selected ? props.colors.selection : undefined}
              onMouseOver={() => setSelected(index)}
              onMouseDown={() => acceptSuggestion(suggestions, index)}
            >
              <text fg={index === selected ? props.colors.accent : props.colors.textMuted}>{index === selected ? "›" : " "}</text>
              <text fg={index === selected ? props.colors.accent : props.colors.text} attributes={TextAttributes.BOLD}>
                {item.command}
              </text>
              <text fg={props.colors.textMuted}>{item.summary}</text>
            </box>
          ))}
        </box>
      )}
      <box
        width="100%"
        border={["left"]}
        borderColor={props.focus === "composer" ? props.colors.accent : props.colors.border}
        backgroundColor={props.colors.panelRaised}
        paddingTop={1}
        paddingBottom={props.roomy ? 1 : 0}
        paddingLeft={2}
        paddingRight={2}
      >
        <textarea
          ref={value => { editor.current = value; }}
          focused={props.focus === "composer"}
          height={props.roomy ? 3 : 2}
          width="100%"
          placeholder={placeholder}
          backgroundColor={props.colors.panelRaised}
          focusedBackgroundColor={props.colors.panelRaised}
          textColor={props.colors.text}
          focusedTextColor={props.colors.text}
          placeholderColor={props.colors.textMuted}
          selectionBg={props.colors.selection}
          selectionFg={props.colors.selectionText}
          keyBindings={[...KEY_BINDINGS]}
          onContentChange={refreshDraft}
          onKeyDown={handleKey}
          onSubmit={submit}
        />
        <box height={1} flexDirection="row" justifyContent="space-between">
          <box flexDirection="row" gap={1}>
            <text fg={props.colors.accent}>{props.busy ? `${props.activityFrame ?? sharedActivityFrame} Working` : "Explore"}</text>
            {minimalChrome ? null : (
              <text fg={props.colors.textMuted}>
                {compactChrome ? `· ${props.workspaceLabel}` : `tenant read-only · ${props.workspaceLabel}`}
              </text>
            )}
          </box>
          <box flexDirection="row" gap={compactChrome ? 1 : 2}>
            {compactChrome ? null : <text fg={props.colors.textMuted}>/ commands</text>}
            <text fg={props.colors.text}>
              Enter{minimalChrome ? null : <> <span style={{fg: props.colors.textMuted}}>send</span></>}
            </text>
          </box>
        </box>
      </box>
    </box>
  );
}
