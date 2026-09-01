// @vitest-environment jsdom

import { useRef } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  TwinDestination,
  useTwinGuidedNavigation,
  type TwinGuideDestination,
} from "./twin-guided-navigation";
import type { TwinWorkspaceTab } from "./twin-workspace-tabs";

function Harness({
  activeTab,
  destinations = [],
  commitTab,
}: {
  activeTab: TwinWorkspaceTab;
  destinations?: readonly TwinGuideDestination[];
  commitTab: (tab: TwinWorkspaceTab) => void;
}) {
  const rootRef = useRef<HTMLDivElement>(null);
  const navigation = useTwinGuidedNavigation({
    activeTab,
    rootRef,
    commitTab,
  });

  return (
    <div ref={rootRef} data-testid="twin-guide-root">
      <button
        type="button"
        onClick={() => navigation.guide({
          kind: "action",
          key: "configure_source",
        })}
      >
        Configure sources
      </button>
      <button
        type="button"
        onClick={() => navigation.guide({
          kind: "inspection",
          key: "execution_evidence",
        })}
      >
        Execution evidence
      </button>
      <button type="button" onClick={() => navigation.selectTab("twin")}>
        Select Twin tab
      </button>
      {destinations.map((destination) => (
        <TwinDestination
          key={destination}
          destination={destination}
          aria-label={destination}
        >
          {destination}
        </TwinDestination>
      ))}
    </div>
  );
}

const rootScrollTo = vi.fn();
const windowScrollTo = vi.fn();

beforeEach(() => {
  rootScrollTo.mockReset();
  windowScrollTo.mockReset();
  Object.defineProperty(HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: rootScrollTo,
  });
  Object.defineProperty(window, "scrollTo", {
    configurable: true,
    value: windowScrollTo,
  });
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: vi.fn(() => ({
      matches: false,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

afterEach(cleanup);

describe("Twin guided navigation", () => {
  it("waits for a delayed cross-tab destination, then scrolls only the Twin root and focuses it", async () => {
    const commitTab = vi.fn();
    const view = render(
      <Harness activeTab="wiki" commitTab={commitTab} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Execution evidence" }));
    expect(commitTab).toHaveBeenCalledWith("use");
    expect(rootScrollTo).not.toHaveBeenCalled();

    view.rerender(<Harness activeTab="use" commitTab={commitTab} />);
    expect(rootScrollTo).not.toHaveBeenCalled();

    view.rerender(
      <Harness
        activeTab="use"
        destinations={["use-effectiveness"]}
        commitTab={commitTab}
      />,
    );

    const destination = screen.getByRole("region", {
      name: "use-effectiveness",
    });
    await waitFor(() => expect(destination).toHaveFocus());
    expect(rootScrollTo).toHaveBeenCalledTimes(1);
    expect(windowScrollTo).not.toHaveBeenCalled();
  });

  it("settles a same-tab destination without replacing the tab and honors reduced motion", async () => {
    vi.mocked(window.matchMedia).mockReturnValue({
      matches: true,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    });
    const commitTab = vi.fn();
    render(
      <Harness
        activeTab="wiki"
        destinations={["wiki-source-policy"]}
        commitTab={commitTab}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Configure sources" }));

    const destination = screen.getByRole("region", {
      name: "wiki-source-policy",
    });
    await waitFor(() => expect(destination).toHaveFocus());
    expect(commitTab).not.toHaveBeenCalled();
    expect(rootScrollTo).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: "auto" }),
    );
  });

  it("focuses the named terminal Use status only when the exact destination cannot render", async () => {
    const commitTab = vi.fn();
    const view = render(
      <Harness activeTab="wiki" commitTab={commitTab} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Execution evidence" }));
    view.rerender(
      <Harness
        activeTab="use"
        destinations={["use-status"]}
        commitTab={commitTab}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("region", { name: "use-status" })).toHaveFocus();
    });
  });

  it("cancels pending guidance when the user selects an ordinary tab", async () => {
    const commitTab = vi.fn();
    const view = render(
      <Harness activeTab="wiki" commitTab={commitTab} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Execution evidence" }));
    fireEvent.click(screen.getByRole("button", { name: "Select Twin tab" }));
    expect(commitTab.mock.calls).toEqual([["use"], ["twin"]]);

    view.rerender(
      <Harness
        activeTab="use"
        destinations={["use-effectiveness"]}
        commitTab={commitTab}
      />,
    );

    await Promise.resolve();
    expect(screen.getByRole("region", {
      name: "use-effectiveness",
    })).not.toHaveFocus();
    expect(rootScrollTo).not.toHaveBeenCalled();
  });

  it("lets the newest guided request win when two actions arrive before the tab commits", async () => {
    const commitTab = vi.fn();
    const view = render(
      <Harness activeTab="wiki" commitTab={commitTab} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Configure sources" }));
    fireEvent.click(screen.getByRole("button", { name: "Execution evidence" }));
    expect(commitTab).toHaveBeenLastCalledWith("use");

    view.rerender(
      <Harness
        activeTab="use"
        destinations={["wiki-source-policy", "use-effectiveness"]}
        commitTab={commitTab}
      />,
    );

    const execution = screen.getByRole("region", {
      name: "use-effectiveness",
    });
    await waitFor(() => expect(execution).toHaveFocus());
    expect(screen.getByRole("region", {
      name: "wiki-source-policy",
    })).not.toHaveFocus();
    expect(rootScrollTo).toHaveBeenCalledTimes(1);
  });

  it("cancels the pending observer when the workspace unmounts", async () => {
    const commitTab = vi.fn();
    const view = render(
      <Harness activeTab="wiki" commitTab={commitTab} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Execution evidence" }));
    view.unmount();
    await Promise.resolve();

    expect(rootScrollTo).not.toHaveBeenCalled();
  });
});
