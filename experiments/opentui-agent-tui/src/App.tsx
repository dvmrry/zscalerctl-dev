import {useKeyboard, useRenderer, useTerminalDimensions} from "@opentui/react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import type {WireValue} from "../../../clients/typescript/src/index.ts";

import {poisonBannerForWidth} from "./brand.ts";
import {
  activeInteractionMode,
  isBusyControl,
  resolveInteractionCommand
} from "./commands.ts";
import {Composer} from "./components/Composer.tsx";
import {ContextRail} from "./components/ContextRail.tsx";
import {Inspector} from "./components/Inspector.tsx";
import {OverlayBackdrop} from "./components/Overlay.tsx";
import type {PickerInputMethod} from "./components/PickerWindow.tsx";
import {SearchBox} from "./components/SearchBox.tsx";
import {Toast} from "./components/Toast.tsx";
import {Transcript} from "./components/Transcript.tsx";
import {WorkingSet} from "./components/WorkingSet.tsx";
import {WorkspacePickerWindow, workspacePickerScopeBarVisible} from "./components/WorkspacePicker.tsx";
import {
  COMMANDS,
  type ContextState,
  type FocusTarget,
  type PinnedEvidence,
  type ResultSnapshot,
  type Tone,
  type TranscriptActionID,
  type TranscriptEvidence,
  type TranscriptEntry
} from "./model.ts";
import {
  DEFAULT_MOTION_MODE,
  isMotionMode,
  motionDescription,
  OPERATION_SCENE_DELAY_MS,
  SYSTEM_MOTION_TIMERS,
  WELCOME_MOTION_DURATION_MS,
  type MotionTimerDriver,
  type MotionMode
} from "./motion.ts";
import {DEFAULT_SPINNER, type SpinnerType} from "./spinner.ts";
import {safeInlineText} from "./text.ts";
import {
  isThemeMode,
  isThemeName,
  modeFromBackground,
  nextTheme,
  nextThemeMode,
  paletteFor,
  THEME_NAMES,
  type ThemeMode,
  type ThemeName
} from "./theme.ts";
import {LatestToastController, type ToastState} from "./toast.ts";
import {resultTranscriptBlocks, snapshotWireValue, textTranscriptBlocks, valueAtPath} from "./transcript.ts";
import {
  expandAncestors,
  flattenTree,
  formatBreadcrumb,
  formatPath,
  initialExpansion,
  parentPath,
  pathKey,
  searchTree,
  toggleExpansion,
  type ArrayOrder,
  type TreeSearchMatch,
  type TreeRow
} from "./tree.ts";
import {MotionProvider} from "./useMotion.ts";
import {
  FIXTURE_WORKSPACE_ADAPTER,
  WorkspaceCommandError,
  filterWorkspacePicker,
  normalizeWorkspacePicker,
  type WorkspaceAdapter,
  type WorkspaceExecutionContext,
  type SafeWorkspacePicker,
  type WorkspacePicker,
  type WorkspaceResult
} from "./workspace.ts";

const EMPTY_IDS: ReadonlySet<string> = new Set<string>();
const EXPERIMENT_THEME_NAMES: ReadonlySet<ThemeName> = new Set<ThemeName>([
  "signal",
  "tron",
  "cyberpunk",
  "mono"
]);

function themePickerFor(currentTheme: ThemeName, mode: ThemeMode): WorkspacePicker {
  const catalogThemes = THEME_NAMES.filter(name => !EXPERIMENT_THEME_NAMES.has(name));
  const experimentThemes = THEME_NAMES.filter(name => EXPERIMENT_THEME_NAMES.has(name));
  const currentGroup = EXPERIMENT_THEME_NAMES.has(currentTheme) ? experimentThemes : catalogThemes;
  const otherGroup = currentGroup === experimentThemes ? catalogThemes : experimentThemes;
  const orderedThemes = [
    currentTheme,
    ...currentGroup.filter(name => name !== currentTheme),
    ...otherGroup
  ];
  return {
    title: "Choose theme",
    placeholder: "Filter themes…",
    instruction: "Type to filter themes, then press Enter to apply one.",
    emptyMessage: "No themes match this search.",
    items: orderedThemes.map(name => {
      const experimentTheme = EXPERIMENT_THEME_NAMES.has(name);
      return {
        id: name,
        title: name,
        description: `${experimentTheme ? "Experimental" : "Curated"} palette · apply in ${mode} mode`,
        searchText: experimentTheme ? "local experimental theme" : "curated theme",
        category: experimentTheme ? "Experimental themes" : "Curated themes",
        ...(name === currentTheme ? {badge: "current"} : {}),
        command: `/theme ${name}`
      };
    })
  };
}

function motionPickerFor(currentMode: MotionMode): WorkspacePicker {
  return {
    title: "Choose motion",
    placeholder: "Filter motion modes…",
    instruction: "Choose how much terminal animation to render.",
    emptyMessage: "No motion modes match this search.",
    items: (["full", "reduced", "off"] as const).map(mode => ({
      id: mode,
      title: mode,
      description: motionDescription(mode),
      searchText: "animation accessibility liveness",
      category: "Motion",
      ...(mode === currentMode ? {badge: "current"} : {}),
      command: `/motion ${mode}`
    }))
  };
}

interface SearchSnapshot {
  readonly selectedId: string;
  readonly expanded: ReadonlySet<string>;
  readonly resultId?: number;
}

type WorkspacePickerPurpose = "workspace" | "theme" | "motion";

function selectedSearchMatch(
  matches: readonly TreeSearchMatch[],
  requestedId: string | undefined
): TreeSearchMatch | undefined {
  return matches.find(match => match.id === requestedId) ?? matches[0];
}

function safeContextState(context: ContextState): ContextState {
  return {
    ...context,
    transport: safeInlineText(context.transport, 160),
    authority: safeInlineText(context.authority, 120),
    scope: safeInlineText(context.scope, 240),
    ...(context.countLabel === undefined ? {} : {countLabel: safeInlineText(context.countLabel, 40)}),
    effects: safeInlineText(context.effects, 240),
    operation: {...context.operation, label: safeInlineText(context.operation.label, 240)}
  };
}

function footerHelpFor(focus: FocusTarget, availableWidth: number): string {
  if (focus === "composer" || focus === "search" || focus === "picker") {
    return availableWidth < 82
      ? "Tab complete/focus · /find · /sidebar"
      : "Tab complete/focus · /find search · /sidebar context · Ctrl+O inspect";
  }
  return availableWidth < 82
    ? "Tab focus · Ctrl+F find · Ctrl+B context"
    : "Tab/Shift+Tab focus · Ctrl+F find · Ctrl+B context · Ctrl+O inspect";
}

export function App(props: {
  readonly initialMode: ThemeMode;
  readonly initialTheme: ThemeName;
  readonly initialMotion?: MotionMode;
  readonly motionTimers?: MotionTimerDriver;
  readonly workspace?: WorkspaceAdapter;
  readonly spinner?: SpinnerType;
}) {
  const renderer = useRenderer();
  const dimensions = useTerminalDimensions();
  const motionTimers = props.motionTimers ?? SYSTEM_MOTION_TIMERS;
  const workspace = props.workspace ?? FIXTURE_WORKSPACE_ADAPTER;
  const initialWorkspace = workspace.initial;
  const initialDataRef = useRef<WireValue | undefined>(undefined);
  if (initialDataRef.current === undefined) initialDataRef.current = snapshotWireValue(initialWorkspace.data);
  const initialData = initialDataRef.current;
  const nextID = useRef(2);
  const nextPinID = useRef(1);
  const nextResultID = useRef(initialWorkspace.context.connection === "connecting" ? 1 : 2);
  const [themeName, setThemeName] = useState(props.initialTheme);
  const [themeMode, setThemeMode] = useState(props.initialMode);
  const [motionMode, setMotionMode] = useState(props.initialMotion ?? DEFAULT_MOTION_MODE);
  const [welcomeMotionActive, setWelcomeMotionActive] = useState(
    (props.initialMotion ?? DEFAULT_MOTION_MODE) === "full"
  );
  const [poisonViewportVisible, setPoisonViewportVisible] = useState(false);
  const [operationSceneVisible, setOperationSceneVisible] = useState(false);
  const colors = useMemo(() => paletteFor(themeName, themeMode), [themeMode, themeName]);
  const [focus, setFocus] = useState<FocusTarget>("composer");
  const [sidebarMode, setSidebarMode] = useState<"auto" | "hide">("auto");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const searchOpenRef = useRef(false);
  const focusRef = useRef<FocusTarget>("composer");
  const searchReturnFocusRef = useRef<FocusTarget>("composer");
  const searchPendingFocusRef = useRef(false);
  const searchSnapshotRef = useRef<SearchSnapshot | undefined>(undefined);
  const [searchQuery, setSearchQuery] = useState("");
  const searchQueryRef = useRef("");
  const [searchSelectedId, setSearchSelectedId] = useState<string | undefined>();
  const searchSelectedIdRef = useRef<string | undefined>(undefined);
  const [searchInputMethod, setSearchInputMethod] = useState<PickerInputMethod>("keyboard");
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const activeOperationRef = useRef<AbortController | undefined>(undefined);
  const quittingRef = useRef(false);
  const submitRef = useRef<(input: string) => void>(() => undefined);
  const workspacePickerPurposeRef = useRef<WorkspacePickerPurpose>("workspace");
  const [workspacePicker, setWorkspacePicker] = useState<SafeWorkspacePicker | undefined>();
  const [workspacePickerQuery, setWorkspacePickerQuery] = useState("");
  const workspacePickerQueryRef = useRef("");
  const [workspacePickerScopeId, setWorkspacePickerScopeId] = useState<string | undefined>();
  const workspacePickerScopeIdRef = useRef<string | undefined>(undefined);
  const [workspacePickerSelectedId, setWorkspacePickerSelectedId] = useState<string | undefined>();
  const workspacePickerSelectedIdRef = useRef<string | undefined>(undefined);
  const [workspacePickerInputMethod, setWorkspacePickerInputMethod] = useState<PickerInputMethod>("keyboard");
  const [arrayOrder, setArrayOrder] = useState<ArrayOrder>("index");
  const [data, setData] = useState(initialData);
  const dataRef = useRef<WireValue>(data);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => initialExpansion(initialData));
  const expandedRef = useRef<ReadonlySet<string>>(expanded);
  const [selectedId, setSelectedId] = useState(pathKey([]));
  const selectedIdRef = useRef(selectedId);
  const [toast, setToast] = useState<ToastState | undefined>();
  const toastController = useRef<LatestToastController | undefined>(undefined);
  const cancellationToastID = useRef<number | undefined>(undefined);
  if (toastController.current === undefined) toastController.current = new LatestToastController(setToast);
  const [context, setContext] = useState<ContextState>(() => safeContextState(initialWorkspace.context));
  const contextRef = useRef<ContextState>(context);
  const initialEntryHasData = initialWorkspace.context.connection !== "connecting";
  const initialResultId = initialEntryHasData ? 1 : undefined;
  const snapshotsRef = useRef<Map<number, ResultSnapshot> | undefined>(undefined);
  if (snapshotsRef.current === undefined) {
    snapshotsRef.current = new Map(initialResultId === undefined ? [] : [[initialResultId, {
      id: initialResultId,
      data: initialData,
      context: safeContextState(initialWorkspace.context)
    }]]);
  }
  const [activeResultId, setActiveResultId] = useState<number | undefined>(initialResultId);
  const activeResultIdRef = useRef<number | undefined>(activeResultId);
  const [pins, setPins] = useState<readonly PinnedEvidence[]>([]);
  const pinsRef = useRef<readonly PinnedEvidence[]>(pins);
  const [entries, setEntries] = useState<readonly TranscriptEntry[]>([
    {
      id: 1,
      role: "assistant",
      title: safeInlineText(initialWorkspace.announcement.title, 180),
      blocks: resultTranscriptBlocks(
        initialWorkspace.announcement.body,
        initialEntryHasData ? initialData : undefined,
        safeContextState(initialWorkspace.context),
        initialWorkspace.announcement.summary
      ),
      tone: initialWorkspace.announcement.tone,
      ...(initialResultId === undefined ? {} : {resultId: initialResultId})
    }
  ]);
  const entriesRef = useRef<readonly TranscriptEntry[]>(entries);

  const commands = useMemo(
    () => [...(workspace.commands ?? []), ...COMMANDS.filter(command => command.command !== "/demo" || workspace.reload !== undefined)],
    [workspace.commands, workspace.reload]
  );
  const wide = dimensions.width > 120;
  const railWidth = Math.max(1, Math.min(48, dimensions.width));
  const sidebarVisible = sidebarOpen || (sidebarMode === "auto" && wide);
  const sidebarOverlay = sidebarVisible && !wide;
  const conversationWidth = dimensions.width - (sidebarVisible && wide ? railWidth : 0);
  const transcriptAvailableWidth = Math.max(20, conversationWidth - 7);
  const compact = dimensions.width < 88 || dimensions.height < 25;
  const artworkVisible = !compact && dimensions.height >= 42;
  const poisonArtworkVisible = artworkVisible
    && poisonBannerForWidth(transcriptAvailableWidth) !== undefined
    && poisonViewportVisible;
  const welcomeSweepVisible = welcomeMotionActive && poisonArtworkVisible;
  const motionActive = busy || welcomeSweepVisible;
  const rows = useMemo(() => flattenTree(data, expanded, {arrayOrder}), [arrayOrder, data, expanded]);
  const searchResult = useMemo(
    () => searchTree(data, searchQuery, {arrayOrder}),
    [arrayOrder, data, searchQuery]
  );
  const searchResultRef = useRef(searchResult);
  const workspacePickerScopeVisible = workspacePicker !== undefined
    && workspacePickerScopeBarVisible(workspacePicker, dimensions.width, dimensions.height);
  const effectiveWorkspacePickerScopeId = workspacePickerScopeVisible ? workspacePickerScopeId : undefined;
  const filteredWorkspacePicker = useMemo(
    () => filterWorkspacePicker(workspacePicker?.items ?? [], workspacePickerQuery, {scopeId: effectiveWorkspacePickerScopeId}),
    [effectiveWorkspacePickerScopeId, workspacePicker, workspacePickerQuery]
  );
  const filteredWorkspacePickerRef = useRef(filteredWorkspacePicker);
  filteredWorkspacePickerRef.current = filteredWorkspacePicker;
  searchResultRef.current = searchResult;
  searchQueryRef.current = searchQuery;
  searchSelectedIdRef.current = searchSelectedId;
  dataRef.current = data;
  selectedIdRef.current = selectedId;
  expandedRef.current = expanded;
  focusRef.current = focus;
  busyRef.current = busy;
  contextRef.current = context;
  activeResultIdRef.current = activeResultId;
  pinsRef.current = pins;
  entriesRef.current = entries;
  workspacePickerQueryRef.current = workspacePickerQuery;
  workspacePickerScopeIdRef.current = effectiveWorkspacePickerScopeId;
  workspacePickerSelectedIdRef.current = workspacePickerSelectedId;
  if (focus === "search") searchPendingFocusRef.current = false;
  const activeMatch = selectedSearchMatch(searchResult.matches, searchSelectedId);
  const activeSearchIndex = activeMatch === undefined
    ? -1
    : searchResult.matches.findIndex(match => match.id === activeMatch.id);
  const matchedIds = useMemo<ReadonlySet<string>>(
    () => searchOpen ? new Set(searchResult.matches.map(match => match.id)) : EMPTY_IDS,
    [searchOpen, searchResult]
  );
  const selectedRow = rows.find(row => row.id === selectedId) ?? rows[0]!;
  const selectedEvidence: TranscriptEvidence = {
    path: selectedRow.path,
    label: safeInlineText(formatBreadcrumb(rows, selectedRow), 80),
    detail: safeInlineText(formatPath(selectedRow.path), 120),
    kind: selectedRow.kind,
    preview: selectedRow.preview
  };
  const footerHelp = footerHelpFor(focus, conversationWidth);
  const breadcrumb = safeInlineText(
    formatBreadcrumb(rows, selectedRow),
    Math.max(10, Math.min(54, conversationWidth - [...footerHelp].length - 6))
  );

  const append = useCallback((entry: Omit<TranscriptEntry, "id">): number => {
    const id = nextID.current++;
    setEntries(current => {
      const next = [...current, {
        ...entry,
        id,
        ...(entry.title === undefined ? {} : {title: safeInlineText(entry.title, 180)})
      }];
      entriesRef.current = next;
      return next;
    });
    return id;
  }, []);

  const showToast = useCallback((message: string, tone: ToastState["tone"]) => {
    return toastController.current?.show(message, tone);
  }, []);

  const presentWorkspacePicker = useCallback((
    source: WorkspacePicker,
    options: {readonly preferredId?: string; readonly purpose?: WorkspacePickerPurpose} = {}
  ) => {
    const picker = normalizeWorkspacePicker(source);
    const query = picker.initialQuery ?? "";
    const filtered = filterWorkspacePicker(picker.items, query);
    const selected = filtered.items.find(item => item.id === options.preferredId) ?? filtered.items[0];
    workspacePickerPurposeRef.current = options.purpose ?? "workspace";
    setWorkspacePicker(picker);
    setWorkspacePickerQuery(query);
    workspacePickerQueryRef.current = query;
    setWorkspacePickerScopeId(undefined);
    workspacePickerScopeIdRef.current = undefined;
    setWorkspacePickerSelectedId(selected?.id);
    workspacePickerSelectedIdRef.current = selected?.id;
    setWorkspacePickerInputMethod("keyboard");
    setFocus("picker");
  }, []);

  const applyWorkspaceResult = useCallback((result: WorkspaceResult) => {
    const resultContext = result.context === undefined
      ? contextRef.current
      : safeContextState(result.context);
    const committedData = result.data === undefined ? undefined : snapshotWireValue(result.data);
    const resultId = committedData === undefined ? undefined : nextResultID.current++;
    if (committedData !== undefined && resultId !== undefined) {
      snapshotsRef.current!.set(resultId, {id: resultId, data: committedData, context: resultContext});
    }
    append({
      role: "assistant",
      title: safeInlineText(result.announcement.title, 180),
      blocks: resultTranscriptBlocks(
        result.announcement.body,
        committedData,
        resultContext,
        result.announcement.summary
      ),
      tone: result.announcement.tone,
      ...(resultId === undefined ? {} : {resultId})
    });
    if (committedData !== undefined) {
      dataRef.current = committedData;
      setData(committedData);
      const nextExpanded = initialExpansion(committedData);
      expandedRef.current = nextExpanded;
      selectedIdRef.current = pathKey([]);
      setExpanded(nextExpanded);
      setSelectedId(selectedIdRef.current);
      activeResultIdRef.current = resultId;
      setActiveResultId(resultId);
      if (searchOpenRef.current) {
        searchSnapshotRef.current = {
          selectedId: selectedIdRef.current,
          expanded: new Set(nextExpanded),
          resultId
        };
        searchSelectedIdRef.current = undefined;
        setSearchSelectedId(undefined);
      }
    }
    if (result.context !== undefined) {
      contextRef.current = resultContext;
      setContext(resultContext);
    }
    if (result.picker !== undefined) presentWorkspacePicker(result.picker);
  }, [append, presentWorkspacePicker]);

  const presentWorkspaceError = useCallback((error: unknown) => {
    const failure = error instanceof WorkspaceCommandError
      ? error
      : new WorkspaceCommandError({title: "Operation failed", message: "The local operation failed unexpectedly."});
    setContext(current => ({
      ...current,
      connection: current.connection === "connecting" ? "error" : current.connection,
      operation: {status: failure.canceled ? "idle" : "error", label: failure.canceled ? "operation canceled" : failure.title.toLowerCase()}
    }));
    append({
      role: "assistant",
      title: safeInlineText(failure.title, 180),
      blocks: textTranscriptBlocks([failure.message, ...failure.details]),
      tone: failure.tone
    });
  }, [append]);

  const runWorkspaceOperation = useCallback(async (
    label: string,
    operation: (context: WorkspaceExecutionContext) => Promise<WorkspaceResult>
  ) => {
    if (busyRef.current || quittingRef.current) return;
    busyRef.current = true;
    setBusy(true);
    const controller = new AbortController();
    let acceptingProgress = true;
    activeOperationRef.current = controller;
    setContext(current => ({...current, operation: {status: "running", label}}));
    try {
      const result = await operation({
        signal: controller.signal,
        emit: event => {
          if (!acceptingProgress || activeOperationRef.current !== controller || controller.signal.aborted) return;
          setContext(current => ({
            ...current,
            operation: {
              status: "running",
              label: safeInlineText(event.message, 240),
              completed: event.completed,
              total: event.total
            }
          }));
        }
      });
      acceptingProgress = false;
      applyWorkspaceResult(result);
      setContext(current => current.operation.status === "running"
        ? {
            ...current,
            operation: {
              status: "complete",
              label: safeInlineText(result.announcement.title, 240)
            }
          }
        : current);
    } catch (error) {
      acceptingProgress = false;
      presentWorkspaceError(error);
    } finally {
      acceptingProgress = false;
      const toastID = cancellationToastID.current;
      if (toastID !== undefined) {
        toastController.current?.dismiss(toastID);
        cancellationToastID.current = undefined;
      }
      if (activeOperationRef.current === controller) activeOperationRef.current = undefined;
      busyRef.current = false;
      setBusy(false);
    }
  }, [applyWorkspaceResult, presentWorkspaceError]);

  const closeWorkspacePicker = useCallback(() => {
    setWorkspacePicker(undefined);
    setWorkspacePickerQuery("");
    workspacePickerQueryRef.current = "";
    setWorkspacePickerScopeId(undefined);
    workspacePickerScopeIdRef.current = undefined;
    setWorkspacePickerSelectedId(undefined);
    workspacePickerSelectedIdRef.current = undefined;
    workspacePickerPurposeRef.current = "workspace";
    setFocus("composer");
  }, []);

  const updateWorkspacePickerQuery = useCallback((value: string) => {
    const query = safeInlineText(value, 120);
    const filtered = filterWorkspacePicker(workspacePicker?.items ?? [], query, {scopeId: workspacePickerScopeIdRef.current});
    const retained = filtered.items.find(item => item.id === workspacePickerSelectedIdRef.current);
    const next = retained ?? filtered.items[0];
    filteredWorkspacePickerRef.current = filtered;
    workspacePickerQueryRef.current = query;
    workspacePickerSelectedIdRef.current = next?.id;
    setWorkspacePickerQuery(query);
    setWorkspacePickerSelectedId(next?.id);
    setWorkspacePickerInputMethod("keyboard");
  }, [workspacePicker]);

  const updateWorkspacePickerScope = useCallback((requestedScopeId: string | undefined) => {
    const scopes = workspacePicker?.scopes ?? [];
    const scopeId = requestedScopeId !== undefined && scopes.some(scope => scope.id === requestedScopeId)
      ? requestedScopeId
      : undefined;
    const filtered = filterWorkspacePicker(workspacePicker?.items ?? [], workspacePickerQueryRef.current, {scopeId});
    const next = filtered.items[0];
    filteredWorkspacePickerRef.current = filtered;
    workspacePickerScopeIdRef.current = scopeId;
    workspacePickerSelectedIdRef.current = next?.id;
    setWorkspacePickerScopeId(scopeId);
    setWorkspacePickerSelectedId(next?.id);
  }, [workspacePicker]);

  const cycleWorkspacePickerScope = useCallback((delta: number) => {
    const scopes = workspacePicker?.scopes ?? [];
    if (scopes.length < 2) return;
    const ids: readonly (string | undefined)[] = [undefined, ...scopes.map(scope => scope.id)];
    const current = ids.findIndex(id => id === workspacePickerScopeIdRef.current);
    const origin = current < 0 ? 0 : current;
    const next = ids[((origin + delta) % ids.length + ids.length) % ids.length];
    updateWorkspacePickerScope(next);
    setWorkspacePickerInputMethod("keyboard");
  }, [updateWorkspacePickerScope, workspacePicker]);

  useEffect(() => {
    if (workspacePicker === undefined || workspacePickerScopeId === undefined || workspacePickerScopeVisible) return;
    updateWorkspacePickerScope(undefined);
  }, [updateWorkspacePickerScope, workspacePicker, workspacePickerScopeId, workspacePickerScopeVisible]);

  const moveWorkspacePicker = useCallback((delta: number, wrap = true) => {
    const items = filteredWorkspacePickerRef.current.items;
    if (items.length === 0) return;
    const current = items.findIndex(item => item.id === workspacePickerSelectedIdRef.current);
    const origin = current < 0 ? 0 : current;
    const requested = origin + delta;
    const index = wrap
      ? ((requested % items.length) + items.length) % items.length
      : Math.max(0, Math.min(items.length - 1, requested));
    const next = items[index];
    workspacePickerSelectedIdRef.current = next?.id;
    setWorkspacePickerSelectedId(next?.id);
  }, []);

  const moveWorkspacePickerToBoundary = useCallback((boundary: "first" | "last") => {
    const items = filteredWorkspacePickerRef.current.items;
    const next = boundary === "first" ? items[0] : items.at(-1);
    workspacePickerSelectedIdRef.current = next?.id;
    setWorkspacePickerSelectedId(next?.id);
  }, []);

  const changeMotionMode = useCallback((mode: MotionMode) => {
    setMotionMode(mode);
    setWelcomeMotionActive(false);
    append({
      role: "assistant",
      title: safeInlineText("Motion changed"),
      blocks: textTranscriptBlocks([`Now using ${mode} motion · ${motionDescription(mode)}.`]),
      tone: "success"
    });
  }, [append]);

  const commitWorkspacePicker = useCallback((id?: string) => {
    const requested = id ?? workspacePickerSelectedIdRef.current;
    const item = filteredWorkspacePickerRef.current.items.find(candidate => candidate.id === requested);
    if (item === undefined) return;
    const purpose = workspacePickerPurposeRef.current;
    closeWorkspacePicker();
    if (purpose === "theme") {
      if (!isThemeName(item.id)) {
        showToast("The selected theme is unavailable.", "warning");
        return;
      }
      setThemeName(item.id);
      append({
        role: "assistant",
        title: safeInlineText("Theme changed"),
        blocks: textTranscriptBlocks([`Now using ${item.id} · ${themeMode}.`]),
        tone: "success"
      });
      return;
    }
    if (purpose === "motion") {
      if (!isMotionMode(item.id)) {
        showToast("The selected motion mode is unavailable.", "warning");
        return;
      }
      changeMotionMode(item.id);
      return;
    }
    submitRef.current(item.command);
  }, [append, changeMotionMode, closeWorkspacePicker, showToast, themeMode]);

  const cancelActiveOperation = useCallback(() => {
    const active = activeOperationRef.current;
    if (active === undefined) {
      showToast("No engine operation is active.", "info");
      return;
    }
    active.abort();
    cancellationToastID.current = showToast("Cancel requested; waiting for engine acknowledgment.", "info");
  }, [showToast]);

  const quit = useCallback(async () => {
    if (quittingRef.current) return;
    quittingRef.current = true;
    activeOperationRef.current?.abort();
    try {
      await workspace.close();
    } catch {
      // Shutdown must restore the terminal even if the child already exited.
    } finally {
      renderer.destroy();
    }
  }, [renderer, workspace]);

  useEffect(() => () => {
    toastController.current?.dispose();
  }, []);

  useEffect(() => {
    if (!welcomeSweepVisible) return;
    let accepting = true;
    const timer = motionTimers.setTimeout(() => {
      if (accepting) setWelcomeMotionActive(false);
    }, WELCOME_MOTION_DURATION_MS);
    return () => {
      accepting = false;
      motionTimers.clearTimeout(timer);
    };
  }, [motionTimers, welcomeSweepVisible]);

  useEffect(() => {
    setOperationSceneVisible(false);
    if (!busy) return;
    let accepting = true;
    const timer = motionTimers.setTimeout(() => {
      if (accepting && busyRef.current) setOperationSceneVisible(true);
    }, OPERATION_SCENE_DELAY_MS);
    return () => {
      accepting = false;
      motionTimers.clearTimeout(timer);
    };
  }, [busy, motionTimers]);

  useEffect(() => {
    const connect = workspace.connect;
    if (connect === undefined) return () => {
      void workspace.close();
    };
    void runWorkspaceOperation("negotiating stdio v1", context => connect(context));
    return () => {
      activeOperationRef.current?.abort();
      void workspace.close();
    };
  }, [runWorkspaceOperation, workspace]);

  const toggleSidebar = useCallback(() => {
    if (sidebarVisible) {
      setSidebarMode("hide");
      setSidebarOpen(false);
      if (focus === "tree") setFocus("composer");
      return;
    }
    if (wide) setSidebarMode("auto");
    else setSidebarOpen(true);
  }, [focus, sidebarVisible, wide]);

  const focusTree = useCallback(() => {
    if (!wide && !sidebarOpen && !inspectorOpen) setSidebarOpen(true);
    if (wide && sidebarMode === "hide" && !inspectorOpen) setSidebarMode("auto");
    setFocus("tree");
  }, [inspectorOpen, sidebarMode, sidebarOpen, wide]);

  const moveFocus = useCallback((direction: 1 | -1) => {
    const order: readonly FocusTarget[] = ["composer", "transcript", "tree"];
    const current = order.indexOf(focusRef.current);
    const origin = current < 0 ? 0 : current;
    const target = order[(origin + direction + order.length) % order.length]!;
    if (target === "tree") focusTree();
    else {
      if (inspectorOpen) setInspectorOpen(false);
      if (!wide && sidebarOpen) setSidebarOpen(false);
      setFocus(target);
    }
  }, [focusTree, inspectorOpen, sidebarOpen, wide]);

  const toggleRow = useCallback((row: TreeRow) => {
    setExpanded(current => toggleExpansion(current, row));
  }, []);

  const toggleArrayOrder = useCallback(() => {
    setArrayOrder(current => current === "index" ? "name" : "index");
  }, []);

  const previewSearchMatch = useCallback((matchId: string): boolean => {
    const match = searchResultRef.current.matches.find(candidate => candidate.id === matchId);
    if (match === undefined) return false;
    const baseExpansion = searchSnapshotRef.current?.expanded ?? expandedRef.current;
    const nextExpanded = expandAncestors(baseExpansion, match.path);
    if (!flattenTree(dataRef.current, nextExpanded, {arrayOrder}).some(row => row.id === match.id)) {
      showToast("That match falls outside the bounded tree view.", "warning");
      return false;
    }
    searchSelectedIdRef.current = match.id;
    setSearchSelectedId(match.id);
    expandedRef.current = nextExpanded;
    selectedIdRef.current = match.id;
    setExpanded(nextExpanded);
    setSelectedId(match.id);
    return true;
  }, [arrayOrder, showToast]);

  const restoreSearchSnapshot = useCallback(() => {
    const snapshot = searchSnapshotRef.current;
    if (snapshot === undefined || snapshot.resultId !== activeResultIdRef.current) return;
    const restoredExpanded = new Set(snapshot.expanded);
    expandedRef.current = restoredExpanded;
    selectedIdRef.current = snapshot.selectedId;
    setExpanded(restoredExpanded);
    setSelectedId(snapshot.selectedId);
  }, []);

  const moveSearchSelection = useCallback((delta: number, wrap = true) => {
    const matches = searchResultRef.current.matches;
    if (matches.length === 0) return;
    const current = matches.findIndex(match => match.id === searchSelectedIdRef.current);
    const origin = current < 0 ? 0 : current;
    const requested = origin + delta;
    const index = wrap
      ? ((requested % matches.length) + matches.length) % matches.length
      : Math.max(0, Math.min(matches.length - 1, requested));
    const match = matches[index];
    if (match !== undefined) previewSearchMatch(match.id);
  }, [previewSearchMatch]);

  const moveSearchToBoundary = useCallback((boundary: "first" | "last") => {
    const matches = searchResultRef.current.matches;
    const match = boundary === "first" ? matches[0] : matches.at(-1);
    if (match !== undefined) previewSearchMatch(match.id);
  }, [previewSearchMatch]);

  const updateSearchQuery = useCallback((value: string) => {
    const query = safeInlineText(value, 120);
    const result = searchTree(data, query, {arrayOrder});
    const retained = result.matches.find(match => match.id === searchSelectedIdRef.current);
    const next = retained ?? result.matches[0];
    searchResultRef.current = result;
    searchQueryRef.current = query;
    setSearchQuery(query);
    setSearchInputMethod("keyboard");
    searchSelectedIdRef.current = next?.id;
    setSearchSelectedId(next?.id);
    if (next === undefined) restoreSearchSnapshot();
    else previewSearchMatch(next.id);
  }, [arrayOrder, data, previewSearchMatch, restoreSearchSnapshot]);

  const openSearch = useCallback((initialQuery?: string) => {
    if (!searchOpenRef.current) {
      searchReturnFocusRef.current = focusRef.current;
      searchSnapshotRef.current = {
        selectedId: selectedIdRef.current,
        expanded: new Set(expandedRef.current),
        resultId: activeResultIdRef.current
      };
    }
    searchOpenRef.current = true;
    searchPendingFocusRef.current = true;
    setSearchOpen(true);
    setSearchInputMethod("keyboard");
    setFocus("search");
    if (initialQuery !== undefined) {
      updateSearchQuery(initialQuery);
      return;
    }
    const matches = searchResultRef.current.matches;
    const retained = matches.find(match => match.id === searchSelectedIdRef.current);
    const next = retained ?? matches[0];
    searchSelectedIdRef.current = next?.id;
    setSearchSelectedId(next?.id);
    if (next !== undefined) previewSearchMatch(next.id);
  }, [previewSearchMatch, updateSearchQuery]);

  const finishSearch = useCallback((restore: boolean, inspect = false) => {
    if (restore) restoreSearchSnapshot();
    searchOpenRef.current = false;
    searchPendingFocusRef.current = false;
    searchSnapshotRef.current = undefined;
    setSearchOpen(false);
    if (inspect) {
      setInspectorOpen(true);
      focusTree();
      return;
    }
    const returnFocus = searchReturnFocusRef.current;
    if (returnFocus === "tree") focusTree();
    else setFocus(returnFocus === "search" ? "composer" : returnFocus);
  }, [focusTree, restoreSearchSnapshot]);

  const cancelSearch = useCallback(() => finishSearch(true), [finishSearch]);

  const activateResultView = useCallback((resultId: number, path: readonly (string | number)[]): boolean => {
    if (busyRef.current) {
      showToast("Wait for the active operation before changing evidence views.", "warning");
      return false;
    }
    const snapshot = snapshotsRef.current!.get(resultId);
    if (snapshot === undefined) {
      showToast("That result snapshot is no longer available.", "warning");
      return false;
    }
    if (valueAtPath(snapshot.data, path) === undefined) {
      showToast("That evidence path is no longer available.", "warning");
      return false;
    }
    const nextExpanded = expandAncestors(initialExpansion(snapshot.data), path);
    const nextSelectedId = pathKey(path);
    if (!flattenTree(snapshot.data, nextExpanded, {arrayOrder}).some(row => row.id === nextSelectedId)) {
      showToast("That evidence falls outside the bounded tree view.", "warning");
      return false;
    }
    if (searchOpenRef.current) cancelSearch();
    if (workspacePicker !== undefined) closeWorkspacePicker();
    dataRef.current = snapshot.data;
    expandedRef.current = nextExpanded;
    selectedIdRef.current = nextSelectedId;
    activeResultIdRef.current = resultId;
    setData(snapshot.data);
    setExpanded(nextExpanded);
    setSelectedId(nextSelectedId);
    setActiveResultId(resultId);
    if (snapshot.context !== undefined) {
      contextRef.current = snapshot.context;
      setContext(snapshot.context);
    }
    return true;
  }, [arrayOrder, cancelSearch, closeWorkspacePicker, showToast, workspacePicker]);

  const revealTranscriptEvidence = useCallback((entry: TranscriptEntry, evidence: TranscriptEvidence) => {
    if (entry.resultId === undefined || !activateResultView(entry.resultId, evidence.path)) return;
    focusTree();
  }, [activateResultView, focusTree]);

  const handleTranscriptAction = useCallback((entry: TranscriptEntry, action: TranscriptActionID) => {
    if (entry.resultId === undefined || !activateResultView(entry.resultId, [])) return;
    if (action === "inspect") {
      setInspectorOpen(true);
      focusTree();
      return;
    }
    setInspectorOpen(false);
    openSearch();
  }, [activateResultView, focusTree, openSearch]);

  const pinEvidence = useCallback((resultId: number, evidence: TranscriptEvidence) => {
    const snapshot = snapshotsRef.current!.get(resultId);
    if (snapshot === undefined || valueAtPath(snapshot.data, evidence.path) === undefined) {
      showToast("That evidence path is no longer available.", "warning");
      return;
    }
    const evidenceKey = pathKey(evidence.path);
    if (pinsRef.current.some(pin => pin.ref.resultId === resultId && pathKey(pin.ref.path) === evidenceKey)) {
      showToast("That evidence is already pinned.", "info");
      return;
    }
    if (pinsRef.current.length >= 8) {
      showToast("The working set is full; remove a pin before adding another.", "warning");
      return;
    }
    const pin: PinnedEvidence = {
      ...evidence,
      id: nextPinID.current++,
      ref: {resultId, path: evidence.path}
    };
    const next = [...pinsRef.current, pin];
    pinsRef.current = next;
    setPins(next);
    showToast(`Pinned ${evidence.label}.`, "success");
  }, [showToast]);

  const pinTranscriptEvidence = useCallback((entry: TranscriptEntry, evidence: TranscriptEvidence) => {
    if (entry.resultId === undefined) return;
    pinEvidence(entry.resultId, evidence);
  }, [pinEvidence]);

  const pinCurrentEvidence = useCallback(() => {
    const resultId = activeResultIdRef.current;
    if (resultId === undefined) {
      showToast("The current engine result has not committed yet.", "warning");
      return;
    }
    pinEvidence(resultId, selectedEvidence);
  }, [pinEvidence, selectedEvidence, showToast]);

  const activatePin = useCallback((pin: PinnedEvidence) => {
    if (!activateResultView(pin.ref.resultId, pin.ref.path)) return;
    focusTree();
  }, [activateResultView, focusTree]);

  const removePin = useCallback((id: number) => {
    const removed = pinsRef.current.find(pin => pin.id === id);
    const next = pinsRef.current.filter(pin => pin.id !== id);
    pinsRef.current = next;
    setPins(next);
    if (removed !== undefined
        && removed.ref.resultId !== activeResultIdRef.current
        && !entriesRef.current.some(entry => entry.resultId === removed.ref.resultId)
        && !next.some(pin => pin.ref.resultId === removed.ref.resultId)) {
      snapshotsRef.current!.delete(removed.ref.resultId);
    }
  }, []);

  const openThemePicker = useCallback(() => {
    if (searchOpenRef.current) cancelSearch();
    presentWorkspacePicker(themePickerFor(themeName, themeMode), {
      preferredId: themeName,
      purpose: "theme"
    });
  }, [cancelSearch, presentWorkspacePicker, themeMode, themeName]);

  const openMotionPicker = useCallback(() => {
    if (searchOpenRef.current) cancelSearch();
    presentWorkspacePicker(motionPickerFor(motionMode), {
      preferredId: motionMode,
      purpose: "motion"
    });
  }, [cancelSearch, motionMode, presentWorkspacePicker]);

  const commitSearch = useCallback((matchId?: string, inspect = false) => {
    const match = selectedSearchMatch(searchResultRef.current.matches, matchId ?? searchSelectedIdRef.current);
    if (match === undefined) return;
    if (!previewSearchMatch(match.id)) return;
    finishSearch(false, inspect);
  }, [finishSearch, previewSearchMatch]);

  const copySearchValue = useCallback((matchId?: string) => {
    const match = selectedSearchMatch(searchResultRef.current.matches, matchId ?? searchSelectedIdRef.current);
    if (match?.copyText === undefined) {
      showToast("Only scalar search results have a copyable value.", "warning");
      return;
    }
    let copied = false;
    try {
      copied = renderer.copyToClipboardOSC52(match.copyText);
    } catch {
      copied = false;
    }
    showToast(copied ? "Copied sanitized value to the terminal clipboard." : "Terminal clipboard copy is unavailable.", copied ? "success" : "warning");
  }, [renderer, showToast]);

  const copySearchPath = useCallback((matchId?: string) => {
    const match = selectedSearchMatch(searchResultRef.current.matches, matchId ?? searchSelectedIdRef.current);
    if (match === undefined) return;
    let copied = false;
    try {
      copied = renderer.copyToClipboardOSC52(formatPath(match.path));
    } catch {
      copied = false;
    }
    showToast(copied ? "Copied exact JSON path to the terminal clipboard." : "Terminal clipboard copy is unavailable.", copied ? "success" : "warning");
  }, [renderer, showToast]);

  const selectRelative = useCallback((delta: number) => {
    const current = Math.max(0, rows.findIndex(row => row.id === selectedId));
    const index = Math.max(0, Math.min(rows.length - 1, current + delta));
    const row = rows[index];
    if (row !== undefined) setSelectedId(row.id);
  }, [rows, selectedId]);

  useKeyboard(event => {
    if (welcomeMotionActive) setWelcomeMotionActive(false);
    const mode = activeInteractionMode({
      search: searchOpenRef.current,
      picker: workspacePicker !== undefined,
      inspector: inspectorOpen,
      drawer: sidebarOverlay
    });
    const command = resolveInteractionCommand(mode, event, focusRef.current);
    if (command === "picker.scope-next" || command === "picker.scope-previous") {
      if (workspacePicker === undefined || !workspacePickerScopeBarVisible(workspacePicker, dimensions.width, dimensions.height)) return;
    }
    if (command !== undefined) {
      event.preventDefault();
      event.stopPropagation();
      switch (command) {
        case "app.interrupt":
          if (activeOperationRef.current !== undefined) cancelActiveOperation();
          else void quit();
          break;
        case "search.toggle":
          if (workspacePicker !== undefined) closeWorkspacePicker();
          if (searchOpenRef.current) cancelSearch();
          else openSearch();
          break;
        case "search.cancel":
          cancelSearch();
          break;
        case "search.commit":
          commitSearch();
          break;
        case "search.inspect":
          commitSearch(undefined, true);
          break;
        case "search.next":
          setSearchInputMethod("keyboard");
          moveSearchSelection(1);
          break;
        case "search.previous":
          setSearchInputMethod("keyboard");
          moveSearchSelection(-1);
          break;
        case "search.page-next":
          setSearchInputMethod("keyboard");
          moveSearchSelection(5, false);
          break;
        case "search.page-previous":
          setSearchInputMethod("keyboard");
          moveSearchSelection(-5, false);
          break;
        case "search.first":
          setSearchInputMethod("keyboard");
          moveSearchToBoundary("first");
          break;
        case "search.last":
          setSearchInputMethod("keyboard");
          moveSearchToBoundary("last");
          break;
        case "search.copy-value":
          copySearchValue(searchSelectedIdRef.current);
          break;
        case "search.copy-path":
          copySearchPath(searchSelectedIdRef.current);
          break;
        case "picker.cancel":
          closeWorkspacePicker();
          break;
        case "picker.commit":
          commitWorkspacePicker();
          break;
        case "picker.next":
          setWorkspacePickerInputMethod("keyboard");
          moveWorkspacePicker(1);
          break;
        case "picker.previous":
          setWorkspacePickerInputMethod("keyboard");
          moveWorkspacePicker(-1);
          break;
        case "picker.page-next":
          setWorkspacePickerInputMethod("keyboard");
          moveWorkspacePicker(6, false);
          break;
        case "picker.page-previous":
          setWorkspacePickerInputMethod("keyboard");
          moveWorkspacePicker(-6, false);
          break;
        case "picker.first":
          moveWorkspacePickerToBoundary("first");
          break;
        case "picker.last":
          moveWorkspacePickerToBoundary("last");
          break;
        case "picker.scope-next":
          cycleWorkspacePickerScope(1);
          break;
        case "picker.scope-previous":
          cycleWorkspacePickerScope(-1);
          break;
        case "sidebar.toggle":
          toggleSidebar();
          break;
        case "inspector.toggle":
          if (inspectorOpen) {
            setInspectorOpen(false);
            setFocus("composer");
          } else {
            setInspectorOpen(true);
            focusTree();
          }
          break;
        case "overlay.close":
          if (mode === "inspector") {
            setInspectorOpen(false);
            setFocus("composer");
          } else if (mode === "drawer") {
            setSidebarOpen(false);
            setFocus("composer");
          } else {
            setFocus("composer");
          }
          break;
        case "focus.next":
          moveFocus(1);
          break;
        case "focus.previous":
          moveFocus(-1);
          break;
      }
      return;
    }
    if (searchPendingFocusRef.current) {
      event.preventDefault();
      event.stopPropagation();
      if (event.name === "backspace") {
        updateSearchQuery([...searchQueryRef.current].slice(0, -1).join(""));
        return;
      }
      const sequence = event.sequence;
      if (!event.ctrl && !event.meta && sequence.length > 0 && !/[\u0000-\u001f\u007f-\u009f]/u.test(sequence)) {
        updateSearchQuery(`${searchQueryRef.current}${sequence}`);
      }
      return;
    }
    if (searchOpenRef.current) return;
    if (focus !== "tree") return;
    const index = Math.max(0, rows.findIndex(row => row.id === selectedId));
    const row = rows[index];
    if (row === undefined) return;
    if (event.name === "/" && !event.ctrl && !event.meta) {
      event.preventDefault();
      openSearch();
      return;
    }
    if (event.name === "s" && !event.ctrl && !event.meta) {
      event.preventDefault();
      toggleArrayOrder();
      return;
    }
    if (event.name === "p" && !event.ctrl && !event.meta && !event.option) {
      event.preventDefault();
      pinCurrentEvidence();
      return;
    }
    if (event.name === "up" || event.name === "down") {
      event.preventDefault();
      selectRelative(event.name === "up" ? -1 : 1);
      return;
    }
    if (event.name === "pageup" || event.name === "pagedown") {
      event.preventDefault();
      selectRelative(event.name === "pageup" ? -8 : 8);
      return;
    }
    if (event.name === "home" || event.name === "end") {
      event.preventDefault();
      const target = event.name === "home" ? rows[0] : rows.at(-1);
      if (target !== undefined) setSelectedId(target.id);
      return;
    }
    if (event.name === "right") {
      event.preventDefault();
      if (row.expandable && !row.expanded) toggleRow(row);
      else if (row.expanded && rows[index + 1]?.depth === row.depth + 1) setSelectedId(rows[index + 1]!.id);
      return;
    }
    if (event.name === "left") {
      event.preventDefault();
      if (row.expanded) toggleRow(row);
      else {
        const parent = rows.find(candidate => candidate.id === pathKey(parentPath(row.path)));
        if (parent !== undefined) setSelectedId(parent.id);
      }
      return;
    }
    if (event.name === "return" || event.name === "kpenter" || event.name === "space") {
      event.preventDefault();
      toggleRow(row);
    }
  });

  const assistant = (title: string, body: readonly string[], tone: Tone = "neutral") => {
    append({
      role: "assistant",
      title: safeInlineText(title, 180),
      blocks: textTranscriptBlocks(body),
      tone
    });
  };

  const handleSubmit = async (input: string) => {
    if (busyRef.current && !isBusyControl(input)) return;
    const tokens = input.trim().split(/\s+/u);
    const command = tokens[0]?.toLowerCase();

    if (command === "/clear" && tokens.length === 1) {
      entriesRef.current = [];
      setEntries([]);
      const retained = new Set<number>(pinsRef.current.map(pin => pin.ref.resultId));
      if (activeResultIdRef.current !== undefined) retained.add(activeResultIdRef.current);
      for (const resultId of snapshotsRef.current!.keys()) {
        if (!retained.has(resultId)) snapshotsRef.current!.delete(resultId);
      }
      return;
    }
    if (command === "/theme" && (tokens[1] === undefined || tokens[1] === "list")) {
      openThemePicker();
      return;
    }
    const motionRequest = command === "/motion" ? tokens[1]?.toLowerCase() : undefined;
    if (command === "/motion" && (motionRequest === undefined || motionRequest === "list")) {
      openMotionPicker();
      return;
    }
    if (command === "/pin" && tokens.length === 1) {
      pinCurrentEvidence();
      return;
    }
    if (command === "/unpin" && (tokens.length === 1 || (tokens.length === 2 && tokens[1] === "all"))) {
      const count = pinsRef.current.length;
      const removedResultIds = new Set(pinsRef.current.map(pin => pin.ref.resultId));
      pinsRef.current = [];
      setPins([]);
      for (const resultId of removedResultIds) {
        if (resultId !== activeResultIdRef.current
            && !entriesRef.current.some(entry => entry.resultId === resultId)) {
          snapshotsRef.current!.delete(resultId);
        }
      }
      showToast(
        count === 0
          ? "The working set is already empty."
          : `${count} pinned evidence item${count === 1 ? "" : "s"} removed.`,
        count === 0 ? "info" : "success"
      );
      return;
    }

    append({role: "user", blocks: textTranscriptBlocks([input])});

    if (command === "/quit" && tokens.length === 1) {
      void quit();
      return;
    }
    if (command === "/cancel" && tokens.length === 1) {
      cancelActiveOperation();
      return;
    }
    if (command === "/help" && tokens.length === 1) {
      assistant("Available commands", commands.map(item => `${item.usage} — ${item.summary}`), "info");
      return;
    }
    if (command === "/find") {
      if (workspacePicker !== undefined) closeWorkspacePicker();
      const query = input.trim().slice(tokens[0]?.length ?? 0).trim();
      openSearch(query.length === 0 ? undefined : query);
      return;
    }
    if (command === "/sidebar" && tokens.length === 1) {
      toggleSidebar();
      assistant("Context rail toggled", ["Use /sidebar while editing; Ctrl+B toggles it from the workspace."], "info");
      return;
    }
    if (command === "/inspect" && tokens.length === 1) {
      setInspectorOpen(value => !value);
      focusTree();
      return;
    }
    if (command === "/theme") {
      const requested = tokens[1];
      if (requested === undefined || requested === "list") return;
      if (requested === "mode") {
        const modeRequest = tokens[2];
        if (modeRequest === undefined) {
          assistant("Appearance mode", [`Current: ${themeMode}`, "Choose auto, dark, light, or toggle."], "info");
          return;
        }
        let selectedMode: ThemeMode;
        if (modeRequest === "toggle") selectedMode = nextThemeMode(themeMode);
        else if (modeRequest === "auto") {
          try {
            const terminal = await renderer.getPalette({size: 16, timeout: 120});
            selectedMode = modeFromBackground(terminal.defaultBackground) ?? themeMode;
          } catch {
            selectedMode = themeMode;
          }
        } else if (isThemeMode(modeRequest)) selectedMode = modeRequest;
        else {
          assistant("Unknown appearance mode", ["Choose auto, dark, light, or toggle."], "warning");
          return;
        }
        setThemeMode(selectedMode);
        assistant("Appearance changed", [`Now using ${themeName} · ${selectedMode}.`], "success");
        return;
      }
      const selected = requested === "next" ? nextTheme(themeName) : requested;
      if (!isThemeName(selected)) {
        assistant("Unknown theme", ["Use /theme list to browse available themes."], "warning");
        return;
      }
      const modeRequest = tokens[2];
      let selectedMode = themeMode;
      if (modeRequest === "auto") {
        try {
          const terminal = await renderer.getPalette({size: 16, timeout: 120});
          selectedMode = modeFromBackground(terminal.defaultBackground) ?? themeMode;
        } catch {
          selectedMode = themeMode;
        }
      } else if (modeRequest !== undefined) {
        if (!isThemeMode(modeRequest)) {
          assistant("Unknown appearance mode", ["Choose auto, dark, or light."], "warning");
          return;
        }
        selectedMode = modeRequest;
      }
      setThemeName(selected);
      setThemeMode(selectedMode);
      assistant("Theme changed", [`Now using ${selected} · ${selectedMode}.`], "success");
      return;
    }
    if (command === "/motion") {
      const requested = motionRequest;
      if (tokens.length !== 2 || requested === undefined || !isMotionMode(requested)) {
        assistant("Unknown motion mode", ["Choose full, reduced, or off; /motion opens the picker."], "warning");
        return;
      }
      changeMotionMode(requested);
      return;
    }
    if (command === "/sort") {
      const requested = tokens[1];
      if (requested === undefined) {
        assistant("Tree order", [`Current: ${arrayOrder}`, "Choose index, name, or toggle."], "info");
        return;
      }
      const selected = requested === "toggle"
        ? arrayOrder === "index" ? "name" : "index"
        : requested;
      if (selected !== "index" && selected !== "name") {
        assistant("Unknown tree order", ["Choose index, name, or toggle."], "warning");
        return;
      }
      setArrayOrder(selected);
      assistant("Tree order changed", [`Named array items now use ${selected} order.`], "success");
      return;
    }
    if (command === "/demo" && tokens.length === 1 && workspace.reload !== undefined) {
      await runWorkspaceOperation("loading sanitized fixture", context => workspace.reload!(context));
      return;
    }
    if (command?.startsWith("/")) {
      if (workspace.execute !== undefined) {
        if (workspacePicker !== undefined) closeWorkspacePicker();
        await runWorkspaceOperation(`executing ${command}`, context => workspace.execute!(input, context));
        return;
      }
      assistant("Unknown command", ["Type / to browse the command palette or use /help."], "warning");
      return;
    }
    assistant(
      "No model attached",
      ["This experiment is only the reusable deterministic shell. Plain-language agent execution is intentionally absent."],
      "info"
    );
  };

  submitRef.current = input => { void handleSubmit(input); };

  const searchSummary = searchOpen
    ? searchResult.matches.length === 0
      ? "find 0"
      : `find ${activeSearchIndex + 1}/${searchResult.matches.length}${searchResult.truncated ? "+" : ""}`
    : undefined;
  const searchWidth = Math.max(32, Math.min(82, dimensions.width - 2));

  const rail = (
    <ContextRail
      colors={colors}
      width={railWidth}
      context={context}
      rows={rows}
      selectedId={selectedRow.id}
      selectedRow={selectedRow}
      focus={focus}
      overlay={sidebarOverlay}
      compact={dimensions.height < 32}
      arrayOrder={arrayOrder}
      searchSummary={searchSummary}
      matchedIds={matchedIds}
      activeMatchId={searchOpen ? activeMatch?.id : undefined}
      onClose={() => {
        if (searchOpenRef.current) cancelSearch();
        setSidebarOpen(false);
        setFocus("composer");
      }}
      onFocusTree={focusTree}
      onSelect={setSelectedId}
      onToggle={toggleRow}
      onToggleOrder={toggleArrayOrder}
      onSearch={() => {
        if (searchOpen) setFocus("search");
        else openSearch();
      }}
      onInspect={() => {
        setInspectorOpen(true);
        focusTree();
      }}
    />
  );

  const shell = (
    <box width={dimensions.width} height={dimensions.height} backgroundColor={colors.background} flexDirection="column">
      <box flexDirection="row" flexGrow={1} minHeight={0}>
        <box
          flexDirection="column"
          flexGrow={1}
          minWidth={0}
          minHeight={0}
          gap={1}
          paddingTop={1}
          paddingBottom={1}
          paddingLeft={2}
          paddingRight={2}
        >
          <Transcript
            colors={colors}
            entries={entries}
            focus={focus}
            compact={compact}
            artwork={artworkVisible}
            availableWidth={transcriptAvailableWidth}
            workspaceLabel={workspace.id === "fixture" ? "fixture" : "stdio v1"}
            context={context}
            operationSceneVisible={operationSceneVisible}
            interactive={!busy}
            onPoisonVisibilityChange={setPoisonViewportVisible}
            onFocus={() => setFocus("transcript")}
            onAction={handleTranscriptAction}
            onEvidence={revealTranscriptEvidence}
            onPinEvidence={pinTranscriptEvidence}
          />
          <WorkingSet
            colors={colors}
            current={selectedEvidence}
            pins={pins}
            availableWidth={Math.max(20, conversationWidth - 6)}
            compact={dimensions.height < 26}
            onPinCurrent={pinCurrentEvidence}
            onActivate={activatePin}
            onRemove={removePin}
          />
          <Composer
            colors={colors}
            focus={focus}
            busy={busy}
            commands={commands}
            workspaceLabel={workspace.id === "fixture" ? "fixture" : "stdio v1"}
            availableWidth={conversationWidth}
            roomy={dimensions.height >= 20}
            onFocus={() => setFocus("composer")}
            onFocusNext={() => moveFocus(1)}
            onSubmit={value => { void handleSubmit(value); }}
          />
          <box height={1} flexShrink={0} flexDirection="row" justifyContent="space-between">
            <text fg={colors.textMuted}>{footerHelp}</text>
            <text fg={colors.textMuted}>{breadcrumb}</text>
          </box>
        </box>
        {sidebarVisible && wide ? rail : null}
      </box>

      {sidebarOverlay ? (
        <OverlayBackdrop
          layer="drawer"
          align="end"
          dim={70}
          contentWidth={railWidth}
          contentHeight="100%"
          onDismiss={() => {
            if (searchOpenRef.current) cancelSearch();
            setSidebarOpen(false);
            setFocus("composer");
          }}
        >
          {rail}
        </OverlayBackdrop>
      ) : null}

      {inspectorOpen ? (
        <Inspector
          colors={colors}
          width={Math.max(34, Math.min(78, dimensions.width - 4))}
          rows={rows}
          selectedId={selectedRow.id}
          selectedRow={selectedRow}
          focus={focus}
          matchedIds={matchedIds}
          activeMatchId={searchOpen ? activeMatch?.id : undefined}
          onClose={() => {
            if (searchOpenRef.current) cancelSearch();
            setInspectorOpen(false);
            setFocus("composer");
          }}
          onFocusTree={focusTree}
          onSelect={setSelectedId}
          onToggle={toggleRow}
        />
      ) : null}

      {searchOpen ? (
        <SearchBox
          colors={colors}
          viewportWidth={dimensions.width}
          viewportHeight={dimensions.height}
          preferredWidth={searchWidth}
          focused={focus === "search"}
          query={searchQuery}
          selectedId={activeMatch?.id}
          matches={searchResult.matches}
          truncated={searchResult.truncated}
          inputMethod={searchInputMethod}
          onInput={updateSearchQuery}
          onFocus={() => {
            searchPendingFocusRef.current = false;
            setFocus("search");
          }}
          onInputMethodChange={setSearchInputMethod}
          onMove={previewSearchMatch}
          onCommit={id => commitSearch(id)}
          onInspect={id => commitSearch(id, true)}
          onCopyValue={copySearchValue}
          onCopyPath={copySearchPath}
          onCancel={cancelSearch}
        />
      ) : null}

      {workspacePicker === undefined ? null : (
        <WorkspacePickerWindow
          colors={colors}
          viewportWidth={dimensions.width}
          viewportHeight={dimensions.height}
          picker={workspacePicker}
          query={workspacePickerQuery}
          items={filteredWorkspacePicker.items}
          activeScopeId={effectiveWorkspacePickerScopeId}
          selectedId={workspacePickerSelectedId}
          truncated={filteredWorkspacePicker.truncated}
          focused={focus === "picker"}
          inputMethod={workspacePickerInputMethod}
          onInput={updateWorkspacePickerQuery}
          onFocus={() => setFocus("picker")}
          onInputMethodChange={setWorkspacePickerInputMethod}
          onMove={id => {
            workspacePickerSelectedIdRef.current = id;
            setWorkspacePickerSelectedId(id);
          }}
          onSelect={commitWorkspacePicker}
          onScopeChange={updateWorkspacePickerScope}
          onCancel={closeWorkspacePicker}
        />
      )}

      {toast === undefined ? null : (
        <Toast
          colors={colors}
          viewportWidth={dimensions.width}
          message={toast.message}
          tone={toast.tone}
        />
      )}
    </box>
  );

  return (
    <MotionProvider
      spinner={props.spinner ?? DEFAULT_SPINNER}
      mode={motionMode}
      active={motionActive}
      timers={motionTimers}
    >
      {shell}
    </MotionProvider>
  );
}
