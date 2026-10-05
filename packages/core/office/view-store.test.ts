// @vitest-environment jsdom

import { afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { useOfficeViewStore } from "./view-store";

const flush = () =>
  new Promise<void>((resolve) => queueMicrotask(() => resolve()));

beforeAll(() => {
  if (typeof globalThis.localStorage?.clear !== "function") {
    const values = new Map<string, string>();
    const storage: Storage = {
      get length() {
        return values.size;
      },
      clear: () => values.clear(),
      getItem: (key) => values.get(key) ?? null,
      key: (index) => Array.from(values.keys())[index] ?? null,
      removeItem: (key) => {
        values.delete(key);
      },
      setItem: (key, value) => {
        values.set(key, value);
      },
    };
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: storage,
    });
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: storage,
    });
  }
});

beforeEach(() => {
  localStorage.clear();
  useOfficeViewStore.setState({ world: "studio" });
  setCurrentWorkspace(null, null);
});

afterEach(() => {
  setCurrentWorkspace(null, null);
});

describe("useOfficeViewStore", () => {
  it("persists only world under the workspace namespace", async () => {
    setCurrentWorkspace("acme", "ws-a");
    await flush();
    useOfficeViewStore.getState().setWorld("expedition");

    const stored = localStorage.getItem("multica_office_view:acme");
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored ?? "{}").state).toEqual({ world: "expedition" });
  });

  it("rehydrates independent worlds per workspace", async () => {
    localStorage.setItem(
      "multica_office_view:acme",
      JSON.stringify({ state: { world: "expedition" }, version: 0 }),
    );
    localStorage.setItem(
      "multica_office_view:beta",
      JSON.stringify({ state: { world: "studio" }, version: 0 }),
    );

    setCurrentWorkspace("acme", "ws-a");
    await flush();
    await flush();
    expect(useOfficeViewStore.getState().world).toBe("expedition");

    setCurrentWorkspace("beta", "ws-b");
    await flush();
    await flush();
    expect(useOfficeViewStore.getState().world).toBe("studio");
  });

  it("recovers an invalid persisted world to studio", async () => {
    localStorage.setItem(
      "multica_office_view:acme",
      JSON.stringify({ state: { world: "future-world" }, version: 0 }),
    );

    setCurrentWorkspace("acme", "ws-a");
    await flush();
    await flush();

    expect(useOfficeViewStore.getState().world).toBe("studio");
  });
});
