// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import enTwins from "../../locales/en/twins.json";
import type { ProjectedItem } from "./twin-workspace-types";

const { copyTextMock, toastErrorMock, toastSuccessMock } = vi.hoisted(() => ({
  copyTextMock: vi.fn(),
  toastErrorMock: vi.fn(),
  toastSuccessMock: vi.fn(),
}));

vi.mock("@multica/ui/lib/clipboard", () => ({
  copyText: copyTextMock,
}));

vi.mock("sonner", () => ({
  toast: { error: toastErrorMock, success: toastSuccessMock },
}));

import { formatTwinSummary, TwinSummaryCopyButton } from "./twin-summary-copy";

const items: readonly ProjectedItem[] = [
  {
    id: "assertion-review",
    title: "Prefer explicit\nreview decisions.",
    summary: "",
    status: "",
    citationKeys: [],
    kind: "assertion",
    applicability: null,
    confidence: null,
    provenance: null,
  },
  {
    id: "assertion-fallback",
    title: "",
    summary: "",
    status: "",
    citationKeys: [],
    kind: "assertion",
    applicability: null,
    confidence: null,
    provenance: null,
  },
];

const expectedSummary = [
  "## Twin v3",
  "",
  "- Prefer explicit review decisions.",
  "- assertion-fallback",
  "",
  "Version digest: `sha256:twin-v3`",
].join("\n");

beforeEach(() => {
  vi.clearAllMocks();
  copyTextMock.mockResolvedValue(true);
});

describe("formatTwinSummary", () => {
  it("formats a pasteable Markdown summary from visible signed-version data", () => {
    expect(formatTwinSummary({
      heading: "Twin v3",
      digestLabel: "Version digest",
      digest: "sha256:twin-v3",
      items,
    })).toBe(expectedSummary);
  });
});

describe("TwinSummaryCopyButton", () => {
  function renderButton() {
    return render(
      <I18nProvider locale="en" resources={{ en: { twins: enTwins } }}>
        <TwinSummaryCopyButton
          heading="Twin v3"
          digest="sha256:twin-v3"
          items={items}
        />
      </I18nProvider>,
    );
  }

  it("copies the selected signed Twin as Markdown and confirms success", async () => {
    renderButton();

    const button = screen.getByRole("button", { name: "Copy summary" });
    expect(button).toHaveClass("h-11");
    fireEvent.click(button);

    expect(copyTextMock).toHaveBeenCalledWith(expectedSummary);
    await waitFor(() => {
      expect(toastSuccessMock).toHaveBeenCalledWith("Twin summary copied");
    });
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it("reports clipboard failures without claiming success", async () => {
    copyTextMock.mockResolvedValue(false);
    renderButton();

    fireEvent.click(screen.getByRole("button", { name: "Copy summary" }));

    await waitFor(() => {
      expect(toastErrorMock).toHaveBeenCalledWith("Failed to copy Twin summary");
    });
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });
});
