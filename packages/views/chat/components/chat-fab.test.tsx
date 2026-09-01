import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: [] }),
}));

vi.mock("@multica/core/chat", () => ({
  useChatStore: (selector: (state: { isOpen: boolean; toggle: () => void }) => unknown) =>
    selector({ isOpen: false, toggle: vi.fn() }),
}));

vi.mock("@multica/core/chat/queries", () => ({
  chatSessionsOptions: () => ({}),
  countUnreadChatSessions: () => 0,
  hasPendingChatTasksOptions: () => ({}),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/logger", () => ({
  createLogger: () => ({ info: vi.fn() }),
}));

vi.mock("@multica/core/shortcuts", () => ({
  useShortcut: () => null,
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children, ...props }: { children: ReactNode } & Record<string, unknown>) => (
    <button type="button" {...props}>{children}</button>
  ),
}));

vi.mock("../../common/shortcut-keycaps", () => ({
  ShortcutKeycaps: () => null,
}));

import { ChatFab } from "./chat-fab";

describe("ChatFab narrow touch target", () => {
  it("keeps the visual circle while exposing a 44px hit area below lg", () => {
    renderWithI18n(<ChatFab />);

    const button = document.querySelector("button[aria-label='Ask Multica']");
    expect(button).not.toBeNull();
    expect(button).toHaveClass(
      "relative",
      "after:absolute",
      "max-lg:after:-inset-2",
      "max-lg:after:rounded-full",
    );
    expect(button).toHaveClass("size-[var(--chat-launcher-size)]");
  });
});
