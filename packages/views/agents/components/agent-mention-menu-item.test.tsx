// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { Agent } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enAgents from "../../locales/en/agents.json";

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

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenuItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
}));

import {
  AgentMentionMenuItem,
  formatAgentMention,
} from "./agent-mention-menu-item";

const agent = {
  id: "agent-1",
  name: "David[TF] (review)",
  archived_at: null,
} as Agent;

function renderItem(value: Agent = agent) {
  return render(
    <I18nProvider locale="en" resources={{ en: { agents: enAgents } }}>
      <AgentMentionMenuItem agent={value} />
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  copyTextMock.mockResolvedValue(true);
});

describe("formatAgentMention", () => {
  it("produces a pasteable mention and escapes Markdown label characters", () => {
    expect(formatAgentMention(agent)).toBe(
      "[@David\\[TF\\] \\(review\\)](mention://agent/agent-1)",
    );
  });
});

describe("AgentMentionMenuItem", () => {
  it("copies a real agent mention and confirms success", async () => {
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: "Copy mention" }));

    expect(copyTextMock).toHaveBeenCalledWith(
      "[@David\\[TF\\] \\(review\\)](mention://agent/agent-1)",
    );
    await waitFor(() => {
      expect(toastSuccessMock).toHaveBeenCalledWith("Mention copied");
    });
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it("reports clipboard failures", async () => {
    copyTextMock.mockResolvedValue(false);
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: "Copy mention" }));

    await waitFor(() => {
      expect(toastErrorMock).toHaveBeenCalledWith("Failed to copy mention");
    });
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });

  it("does not offer an unusable mention for archived agents", () => {
    renderItem({ ...agent, archived_at: "2026-08-26T00:00:00Z" });

    expect(
      screen.queryByRole("button", { name: "Copy mention" }),
    ).not.toBeInTheDocument();
  });
});
