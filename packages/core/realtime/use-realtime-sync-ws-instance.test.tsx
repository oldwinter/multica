/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { WSClient } from "../api/ws-client";
import { defaultStorage } from "../platform/storage";
import { issueKeys } from "../issues/queries";
import { roomKeys } from "../rooms";
import { chatKeys } from "../chat/queries";
import { workspaceWorkingAgentsKeys } from "../agents/queries";
import { workspaceKeys } from "../workspace/queries";
import { issueStatusKeys } from "../issue-statuses/queries";
import { officeKeys } from "../office/queries";
import { wikiKeys as workspaceWikiKeys } from "../wiki/queries";
import { wikiKeys as lmWikiKeys } from "../twins/queries";
import type { Issue } from "../types";
import {
  markWorkspaceDeletePending,
  unmarkWorkspaceDeletePending,
} from "../workspace/pending-delete";
import { useRealtimeSync, type RealtimeSyncStores } from "./use-realtime-sync";

vi.mock("../platform/workspace-storage", () => ({
  getCurrentWsId: () => "ws-1",
  getCurrentSlug: () => "test-ws",
  // Draft stores are now loaded transitively (storage-cleanup → register-all-drafts)
  // so their persist wiring must resolve against this mock.
  createWorkspaceAwareStorage: (adapter: unknown) => adapter,
  registerForWorkspaceRehydration: () => {},
}));

vi.mock("../paths", () => ({
  useHasOnboarded: () => true,
  resolvePostAuthDestination: () => "/",
}));

function createMockWs(): WSClient {
  return {
    on: vi.fn(() => () => {}),
    onAny: vi.fn(() => () => {}),
    onReconnect: vi.fn(() => () => {}),
  } as unknown as WSClient;
}

function createStores(): RealtimeSyncStores {
  return {
    authStore: Object.assign(() => ({}), {
      getState: () => ({ user: { id: "u1" } }),
      subscribe: () => () => {},
      setState: () => {},
      destroy: () => {},
    }),
  } as unknown as RealtimeSyncStores;
}

function createWrapper(qc: QueryClient) {
  // Named function (not arrow) so react/display-name lint rule passes —
  // anonymous render-fn components break that rule even in test files.
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

function makeIssue(): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue One",
    description: null,
    status: "todo",
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "member-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
  };
}

describe("useRealtimeSync — ws instance change", () => {
  let qc: QueryClient;
  let stores: RealtimeSyncStores;
  let invalidateSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    stores = createStores();
    invalidateSpy = vi.spyOn(qc, "invalidateQueries");
  });

  it("skips invalidation on first non-null ws instance", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    // The main effect calls invalidateQueries for its own setup, but the
    // ws-instance-change effect should NOT have fired invalidation.
    // The only invalidateQueries calls should come from the main effect's
    // event handlers, not from the instance-change effect.
    // We verify by checking that no call was made with workspaceKeys.list()
    // pattern from the instance-change path (it logs a specific message).
    // Simpler: count calls — first mount with a ws should not trigger the
    // workspace-scoped bulk invalidation.
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("does not invalidate when ws goes from instance to null", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("invalidates Office Issue briefs once when a new ws instance appears after null gap", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });
    expect(invalidateSpy).not.toHaveBeenCalled();

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    const targetKey = JSON.stringify(officeKeys.issueBriefsAll("ws-1"));
    const officeInvalidations = invalidateSpy.mock.calls.filter(
      (call: [{ queryKey?: unknown }, ...unknown[]]) =>
        JSON.stringify(call[0].queryKey) === targetKey,
    );
    expect(officeInvalidations).toHaveLength(1);
  });

  it("does not re-invalidate when rerendered with the same ws instance", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    // Rerender with same instance
    rerender({ ws: ws1 });

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("invalidates chat, pins, labels, and invitations queries on ws instance change", () => {
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    const calls = invalidateSpy.mock.calls.map((call: [{ queryKey?: unknown }, ...unknown[]]) => call[0].queryKey);
    expect(calls).toContainEqual(["chat", "ws-1"]);
    expect(calls).toContainEqual(["labels", "ws-1"]);
    expect(calls).toContainEqual(["workspaces", "ws-1", "invitations"]);
    expect(calls).toContainEqual(roomKeys.all("ws-1"));
    expect(calls).toContainEqual(workspaceWikiKeys.all("ws-1"));
    expect(calls).toContainEqual(lmWikiKeys.all("ws-1"));
    // A catalog edit made while this client was disconnected is otherwise
    // invisible for the query's whole 5-minute staleTime.
    expect(calls).toContainEqual(issueStatusKeys.all("ws-1"));
    expect(calls).toContainEqual(officeKeys.issueBriefsAll("ws-1"));
  });

  it("invalidates per-issue caches (no wsId in key) on ws instance change", () => {
    // These keys are not under the ["issues", wsId] prefix, so they need
    // their own invalidation on recovery — otherwise events missed while
    // disconnected leave them stale forever (staleTime: Infinity, #3953).
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    const calls = invalidateSpy.mock.calls.map((call: [{ queryKey?: unknown }, ...unknown[]]) => call[0].queryKey);
    expect(calls).toContainEqual(["issues", "timeline"]);
    expect(calls).toContainEqual(["issues", "reactions"]);
    expect(calls).toContainEqual(["issues", "subscribers"]);
    expect(calls).toContainEqual(["issues", "usage"]);
    expect(calls).toContainEqual(["issues", "attachments"]);
    expect(calls).toContainEqual(["issues", "tasks"]);
  });

  it("invalidates per-chat-session caches (no wsId in key) on ws instance change", () => {
    // These keys are not under the ["chat", wsId] prefix, so they need their
    // own recovery invalidation when reconnecting after missed chat/task events.
    const ws1 = createMockWs();
    const { rerender } = renderHook(
      ({ ws }) => useRealtimeSync(ws, stores),
      { initialProps: { ws: ws1 as WSClient | null }, wrapper: createWrapper(qc) },
    );

    invalidateSpy.mockClear();
    rerender({ ws: null });

    const ws2 = createMockWs();
    rerender({ ws: ws2 });

    const calls = invalidateSpy.mock.calls.map((call: [{ queryKey?: unknown }, ...unknown[]]) => call[0].queryKey);
    expect(calls).toContainEqual(["chat", "messages"]);
    expect(calls).toContainEqual(["chat", "messages-page"]);
    expect(calls).toContainEqual(["chat", "pending-task"]);
    expect(calls).toContainEqual(["task-messages"]);
  });

  it("invalidates per-chat-session caches after an established ws reconnects", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const reconnect = vi.mocked(ws.onReconnect).mock.calls[0]?.[0];
    expect(reconnect).toBeDefined();

    invalidateSpy.mockClear();
    reconnect!();

    const calls = invalidateSpy.mock.calls.map((call: [{ queryKey?: unknown }, ...unknown[]]) => call[0].queryKey);
    expect(calls).toContainEqual(chatKeys.messagesAll());
    expect(calls).toContainEqual(chatKeys.messagesPageAll());
    expect(calls).toContainEqual(chatKeys.pendingTaskAll());
    expect(calls).toContainEqual(officeKeys.issueBriefsAll("ws-1"));
  });

  it.each([
    ["issue:created", { issue: makeIssue() }],
    ["issue:updated", { issue: makeIssue() }],
    ["issue:deleted", { issue_id: "issue-1" }],
  ])("invalidates Office Issue briefs on %s", (event, payload) => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const handler = vi
      .mocked(ws.on)
      .mock.calls.find(([eventType]) => eventType === event)?.[1];
    expect(handler).toBeDefined();

    invalidateSpy.mockClear();
    (handler as (value: unknown) => void)(payload);

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: officeKeys.issueBriefsAll("ws-1"),
    });
    expect(invalidateSpy).not.toHaveBeenCalledWith({
      queryKey: officeKeys.issueBriefsAll("ws-other"),
    });
  });

  it("invalidates one issue attachment cache after detached channel media binds", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const attachmentChanged = vi
      .mocked(ws.on)
      .mock.calls.find(([event]) => event === "issue_attachments:changed")?.[1];
    expect(attachmentChanged).toBeDefined();

    (attachmentChanged as (payload: unknown) => void)({ issue_id: "issue-1" });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: issueKeys.attachments("issue-1"),
    });
  });
  it("refetches the status catalog after an admin changes it elsewhere", async () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const onAny = vi.mocked(ws.onAny).mock.calls[0]?.[0];
    expect(onAny).toBeDefined();

    onAny!({ type: "issue_status:changed", payload: { action: "created" } } as never);
    await new Promise((resolve) => setTimeout(resolve, 120));

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: issueStatusKeys.all("ws-1"),
    });
    // Deliberately NOT the issue caches. A row stores the status KEY; its name,
    // color and category are resolved from the catalog at render time, so no
    // cached issue field can go stale here. Dragging every board and list along
    // would turn one admin rename into a workspace-wide refetch storm on every
    // connected client. (MUL-6458)
    expect(invalidateSpy).not.toHaveBeenCalledWith({
      queryKey: issueKeys.all("ws-1"),
    });
  });

  it("ignores removed DingTalk group-route events", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const onAny = vi.mocked(ws.onAny).mock.calls[0]?.[0];
    expect(onAny).toBeDefined();

    onAny!({ type: "dingtalk_group_route:updated", payload: {} } as never);

    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});

describe("useRealtimeSync — queued chat promotion", () => {
  it("refetches the transcript when a queued prompt starts running", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const ws = createMockWs();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    renderHook(() => useRealtimeSync(ws, createStores()), {
      wrapper: createWrapper(qc),
    });
    const dispatch = vi
      .mocked(ws.on)
      .mock.calls.find(([event]) => event === "task:dispatch")?.[1];
    expect(dispatch).toBeDefined();

    invalidate.mockClear();
    (dispatch as (payload: unknown) => void)({
      task_id: "task-follow-up",
      chat_session_id: "session-1",
    });

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: chatKeys.messages("session-1"),
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: chatKeys.messagesPage("session-1"),
    });
  });
});

describe("useRealtimeSync — Table server membership invalidation", () => {
  let qc: QueryClient;
  let stores: RealtimeSyncStores;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    stores = createStores();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("invalidates Table queries after a task lifecycle event", () => {
    vi.useFakeTimers();
    const ws = createMockWs();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const onAny = vi.mocked(ws.onAny).mock.calls[0]?.[0];
    expect(onAny).toBeDefined();

    onAny!({ type: "task:completed", payload: {} } as never);
    vi.advanceTimersByTime(100);

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: issueKeys.tableAll("ws-1"),
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceWorkingAgentsKeys.all("ws-1"),
    });
  });

  it("invalidates Room queries after a malformed Room entry event", () => {
    const ws = createMockWs();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const roomEntry = vi.mocked(ws.on).mock.calls
      .find(([event]) => event === "room:entry")?.[1];
    expect(roomEntry).toBeDefined();

    roomEntry!({});

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: roomKeys.all("ws-1"),
    });
  });

  it("routes known Wiki events through the targeted handler", () => {
    const ws = createMockWs();
    const detailKey = workspaceWikiKeys.detail("ws-1", "page-1");
    qc.setQueryData(detailKey, {});
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const pageUpdated = vi
      .mocked(ws.on)
      .mock.calls.find(([event]) => event === "wiki:page_updated")?.[1];
    expect(pageUpdated).toBeDefined();

    (pageUpdated as (payload: unknown) => void)({
      page_id: "page-1",
      scope: "workspace",
      revision_id: "revision-2",
      revision_number: 2,
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
  });

  it("uses the Wiki prefix fallback for an unknown future event", () => {
    vi.useFakeTimers();
    const ws = createMockWs();
    const listKey = workspaceWikiKeys.list("ws-1", { scope: "workspace" });
    qc.setQueryData(listKey, []);
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const onAny = vi.mocked(ws.onAny).mock.calls[0]?.[0];
    expect(onAny).toBeDefined();

    onAny!({ type: "wiki:future_lifecycle", payload: {} } as never);
    vi.advanceTimersByTime(100);

    expect(qc.getQueryState(listKey)?.isInvalidated).toBe(true);
  });

  it("registers the recommendation review handler", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });

    expect(vi.mocked(ws.on).mock.calls.some(
      ([event]) => event === "room:recommendation_review",
    )).toBe(true);
  });

  it("invalidates Table queries after a property definition changes", () => {
    const ws = createMockWs();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    const propertyUpdated = vi
      .mocked(ws.on)
      .mock.calls.find(([event]) => event === "property:updated")?.[1];
    expect(propertyUpdated).toBeDefined();

    (propertyUpdated as (payload: unknown) => void)({});

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: issueKeys.tableAll("ws-1"),
    });
  });
});

describe("useRealtimeSync — workspace:deleted self-initiated suppression", () => {
  let qc: QueryClient;
  let stores: RealtimeSyncStores;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    stores = createStores();
  });

  afterEach(() => {
    unmarkWorkspaceDeletePending("ws-2");
    localStorage.clear();
  });

  // getCurrentWsId is mocked to "ws-1" at module level, so deleting "ws-2"
  // never enters the relocate branch — these tests only exercise the
  // storage-cleanup path, which is the observable difference between a
  // handled and a suppressed event.
  const dispatchWorkspaceDeleted = (ws: WSClient, workspaceId: string) => {
    const call = vi
      .mocked(ws.on)
      .mock.calls.find(([event]) => event === "workspace:deleted");
    expect(call).toBeDefined();
    (call![1] as (p: unknown) => void)({ workspace_id: workspaceId });
  };

  it("ignores the event for a delete this client initiated", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    qc.setQueryData(workspaceKeys.list(), [{ id: "ws-2", slug: "delete-me" }]);
    defaultStorage.setItem("multica_issue_draft:delete-me", "draft");

    markWorkspaceDeletePending("ws-2");
    dispatchWorkspaceDeleted(ws, "ws-2");

    // useDeleteWorkspace.onSuccess owns cleanup for self-initiated deletes;
    // the handler must not have touched storage.
    expect(defaultStorage.getItem("multica_issue_draft:delete-me")).toBe("draft");
  });

  it("still cleans up for a delete initiated elsewhere", () => {
    const ws = createMockWs();
    renderHook(() => useRealtimeSync(ws, stores), {
      wrapper: createWrapper(qc),
    });
    qc.setQueryData(workspaceKeys.list(), [{ id: "ws-2", slug: "delete-me" }]);
    defaultStorage.setItem("multica_issue_draft:delete-me", "draft");

    dispatchWorkspaceDeleted(ws, "ws-2");

    expect(defaultStorage.getItem("multica_issue_draft:delete-me")).toBeNull();
  });
});
