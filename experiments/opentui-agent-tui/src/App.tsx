import {useKeyboard, useRenderer, useTerminalDimensions} from "@opentui/react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import type {WireValue} from "../../../clients/typescript/src/index.ts";

import {
  activeInteractionMode,
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
import {WorkspacePickerWindow, workspacePickerScopeBarVisible} from "./components/WorkspacePicker.tsx";
import {
  COMMANDS,
  type ContextState,
  type FocusTarget,
  type Tone,
  type TranscriptEntry
} from "./model.ts";
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
  type TreeRow
} from "./tree.ts";
import {
  FIXTURE_WORKSPACE_ADAPTER,
  WorkspaceCommandError,
  filterWorkspacePicker,
  normalizeWorkspacePicker,
  type WorkspaceAdapter,
  type WorkspaceExecutionContext,
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

interface SearchSnapshot {
  readonly selectedId: string;
  readonly expanded: ReadonlySet<string>;
}

type WorkspacePickerPurpose = "workspace" | "theme";

function safeContextState(context: ContextState): ContextState {
  return {
    ...context,
    transport: safeInlineText(context.transport, 160),
    authority: safeInlineText(context.authority, 120),
    scope: safeInlineText(context.scope, 240),
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
  readonly workspace?: WorkspaceAdapter;
}) {
  const renderer = useRenderer();
  const dimensions = useTerminalDimensions();
  const workspace = props.workspace ?? FIXTURE_WORKSPACE_ADAPTER;
  const initialWorkspace = workspace.initial;
  const nextID = useRef(2);
  const [themeName, setThemeName] = useState(props.initialTheme);
  const [themeMode, setThemeMode] = useState(props.initialMode);
  const colors = paletteFor(themeName, themeMode);
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
  const [workspacePicker, setWorkspacePicker] = useState<WorkspacePicker | undefined>();
  const [workspacePickerQuery, setWorkspacePickerQuery] = useState("");
  const workspacePickerQueryRef = useRef("");
  const [workspacePickerScopeId, setWorkspacePickerScopeId] = useState<string | undefined>();
  const workspacePickerScopeIdRef = useRef<string | undefined>(undefined);
  const [workspacePickerSelectedId, setWorkspacePickerSelectedId] = useState<string | undefined>();
  const workspacePickerSelectedIdRef = useRef<string | undefined>(undefined);
  const [workspacePickerInputMethod, setWorkspacePickerInputMethod] = useState<PickerInputMethod>("keyboard");
  const [arrayOrder, setArrayOrder] = useState<ArrayOrder>("index");
  const [data, setData] = useState(initialWorkspace.data);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => initialExpansion(initialWorkspace.data));
  const expandedRef = useRef<ReadonlySet<string>>(expanded);
  const [selectedId, setSelectedId] = useState(pathKey([]));
  const selectedIdRef = useRef(selectedId);
  const [toast, setToast] = useState<ToastState | undefined>();
  const toastController = useRef<LatestToastController | undefined>(undefined);
  if (toastController.current === undefined) toastController.current = new LatestToastController(setToast);
  const [context, setContext] = useState<ContextState>(() => safeContextState(initialWorkspace.context));
  const [entries, setEntries] = useState<readonly TranscriptEntry[]>([
    {
      id: 1,
      role: "assistant",
      title: safeInlineText(initialWorkspace.announcement.title, 180),
      body: initialWorkspace.announcement.body.map(line => safeInlineText(line, 4_096)),
      tone: initialWorkspace.announcement.tone,
      ...(initialWorkspace.context.connection === "connecting" ? {} : {data: initialWorkspace.data})
    }
  ]);

  const commands = useMemo(
    () => [...(workspace.commands ?? []), ...COMMANDS.filter(command => command.command !== "/demo" || workspace.reload !== undefined)],
    [workspace.commands, workspace.reload]
  );
  const wide = dimensions.width > 120;
  const railWidth = Math.max(1, Math.min(48, dimensions.width));
  const sidebarVisible = sidebarOpen || (sidebarMode === "auto" && wide);
  const sidebarOverlay = sidebarVisible && !wide;
  const conversationWidth = dimensions.width - (sidebarVisible && wide ? railWidth : 0);
  const compact = dimensions.width < 88 || dimensions.height < 25;
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
  selectedIdRef.current = selectedId;
  expandedRef.current = expanded;
  focusRef.current = focus;
  busyRef.current = busy;
  workspacePickerQueryRef.current = workspacePickerQuery;
  workspacePickerScopeIdRef.current = effectiveWorkspacePickerScopeId;
  workspacePickerSelectedIdRef.current = workspacePickerSelectedId;
  if (focus === "search") searchPendingFocusRef.current = false;
  const requestedSearchIndex = searchResult.matches.findIndex(match => match.id === searchSelectedId);
  const activeSearchIndex = requestedSearchIndex < 0 ? 0 : requestedSearchIndex;
  const activeMatch = searchResult.matches[activeSearchIndex];
  const matchedIds = useMemo<ReadonlySet<string>>(
    () => searchOpen ? new Set(searchResult.matches.map(match => match.id)) : EMPTY_IDS,
    [searchOpen, searchResult]
  );
  const selectedRow = rows.find(row => row.id === selectedId) ?? rows[0]!;
  const footerHelp = footerHelpFor(focus, conversationWidth);
  const breadcrumb = safeInlineText(
    formatBreadcrumb(rows, selectedRow),
    Math.max(10, Math.min(54, conversationWidth - [...footerHelp].length - 6))
  );

  const append = useCallback((entry: Omit<TranscriptEntry, "id">) => {
    const id = nextID.current++;
    setEntries(current => [...current, {
      ...entry,
      id,
      ...(entry.title === undefined ? {} : {title: safeInlineText(entry.title, 180)}),
      body: entry.body.map(line => safeInlineText(line, 4_096))
    }]);
  }, []);

  const showToast = useCallback((message: string, tone: ToastState["tone"]) => {
    toastController.current?.show(message, tone);
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
    if (result.data !== undefined) {
      setData(result.data);
      setExpanded(initialExpansion(result.data));
      setSelectedId(pathKey([]));
    }
    if (result.context !== undefined) setContext(safeContextState(result.context));
    append({
      role: "assistant",
      ...result.announcement,
      ...(result.data === undefined ? {} : {data: result.data})
    });
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
      title: failure.title,
      body: [failure.message, ...failure.details],
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
        title: "Theme changed",
        body: [`Now using ${item.id} · ${themeMode}.`],
        tone: "success"
      });
      return;
    }
    submitRef.current(item.command);
  }, [append, closeWorkspacePicker, showToast, themeMode]);

  const cancelActiveOperation = useCallback(() => {
    const active = activeOperationRef.current;
    if (active === undefined) {
      showToast("No engine operation is active.", "info");
      return;
    }
    active.abort();
    showToast("Cancel requested; waiting for engine acknowledgment.", "info");
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

  const previewSearchMatch = useCallback((matchId: string) => {
    const match = searchResultRef.current.matches.find(candidate => candidate.id === matchId);
    if (match === undefined) return;
    searchSelectedIdRef.current = match.id;
    setSearchSelectedId(match.id);
    const baseExpansion = searchSnapshotRef.current?.expanded ?? expandedRef.current;
    setExpanded(expandAncestors(baseExpansion, match.path));
    setSelectedId(match.id);
  }, []);

  const restoreSearchSnapshot = useCallback(() => {
    const snapshot = searchSnapshotRef.current;
    if (snapshot === undefined) return;
    setExpanded(new Set(snapshot.expanded));
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
    else {
      const baseExpansion = searchSnapshotRef.current?.expanded ?? expandedRef.current;
      setExpanded(expandAncestors(baseExpansion, next.path));
      setSelectedId(next.id);
    }
  }, [arrayOrder, data, restoreSearchSnapshot]);

  const openSearch = useCallback((initialQuery?: string) => {
    if (!searchOpenRef.current) {
      searchReturnFocusRef.current = focusRef.current;
      searchSnapshotRef.current = {
        selectedId: selectedIdRef.current,
        expanded: new Set(expandedRef.current)
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

  const openThemePicker = useCallback(() => {
    if (searchOpenRef.current) cancelSearch();
    presentWorkspacePicker(themePickerFor(themeName, themeMode), {
      preferredId: themeName,
      purpose: "theme"
    });
  }, [cancelSearch, presentWorkspacePicker, themeMode, themeName]);

  const commitSearch = useCallback((matchId?: string, inspect = false) => {
    const requested = matchId ?? searchSelectedIdRef.current;
    if (requested === undefined) return;
    const match = searchResultRef.current.matches.find(candidate => candidate.id === requested);
    if (match === undefined) return;
    previewSearchMatch(match.id);
    finishSearch(false, inspect);
  }, [finishSearch, previewSearchMatch]);

  const copySearchValue = useCallback((matchId?: string) => {
    const match = searchResultRef.current.matches.find(candidate => candidate.id === matchId);
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
    const match = searchResultRef.current.matches.find(candidate => candidate.id === matchId);
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

  const assistant = (title: string, body: readonly string[], tone: Tone = "neutral", attachedData?: WireValue) => {
    append({role: "assistant", title, body, tone, ...(attachedData === undefined ? {} : {data: attachedData})});
  };

  const handleSubmit = async (input: string) => {
    const controlWhileBusy = /^\/(?:cancel|quit)$/iu.test(input.trim());
    if (busyRef.current && !controlWhileBusy) return;
    const tokens = input.trim().split(/\s+/u);
    const command = tokens[0]?.toLowerCase();

    if (command === "/clear" && tokens.length === 1) {
      setEntries([]);
      return;
    }
    if (command === "/theme" && (tokens[1] === undefined || tokens[1] === "list")) {
      openThemePicker();
      return;
    }

    append({role: "user", body: [input]});

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

  return (
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
            workspaceLabel={workspace.id === "fixture" ? "fixture" : "stdio v1"}
            onFocus={() => setFocus("transcript")}
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
}
