// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { SkillSummary } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enSkills from "../../locales/en/skills.json";

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
  formatSkillReference,
  SkillReferenceMenuItem,
} from "./skill-reference-copy";

const skill = {
  id: "skill-1",
  name: "deploy[prod] (safe)",
} as SkillSummary;

function renderItem() {
  return render(
    <I18nProvider locale="en" resources={{ en: { skills: enSkills } }}>
      <SkillReferenceMenuItem skill={skill} />
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  copyTextMock.mockResolvedValue(true);
});

describe("formatSkillReference", () => {
  it("produces a pasteable slash-command reference with an escaped label", () => {
    expect(formatSkillReference(skill)).toBe(
      "[/deploy\\[prod\\] \\(safe\\)](slash://skill/skill-1)",
    );
  });
});

describe("SkillReferenceMenuItem", () => {
  it("copies the skill reference and confirms success", async () => {
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: "Copy reference" }));

    expect(copyTextMock).toHaveBeenCalledWith(
      "[/deploy\\[prod\\] \\(safe\\)](slash://skill/skill-1)",
    );
    await waitFor(() => {
      expect(toastSuccessMock).toHaveBeenCalledWith("Skill reference copied");
    });
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it("reports clipboard failures", async () => {
    copyTextMock.mockResolvedValue(false);
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: "Copy reference" }));

    await waitFor(() => {
      expect(toastErrorMock).toHaveBeenCalledWith(
        "Failed to copy skill reference",
      );
    });
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });
});
