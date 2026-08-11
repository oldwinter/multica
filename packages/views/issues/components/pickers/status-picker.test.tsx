// @vitest-environment jsdom

import { cleanup, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../../test/i18n";
import { StatusPicker } from "./status-picker";

const reducedMotion = { value: false };
const animations: EventTarget[] = [];
const animate = vi.fn(() => {
  const animation = new EventTarget();
  animations.push(animation);
  return animation;
});
const originalAnimate = Object.getOwnPropertyDescriptor(Element.prototype, "animate");

vi.mock("motion/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("motion/react")>()),
  useReducedMotion: () => reducedMotion.value,
}));

beforeEach(() => {
  reducedMotion.value = false;
  animate.mockClear();
  animations.length = 0;
  Object.defineProperty(Element.prototype, "animate", {
    configurable: true,
    value: animate,
  });
});

afterEach(() => {
  cleanup();
  document.querySelectorAll("[data-completion-burst]").forEach((burst) => burst.remove());
  if (originalAnimate) {
    Object.defineProperty(Element.prototype, "animate", originalAnimate);
  } else {
    Reflect.deleteProperty(Element.prototype, "animate");
  }
});

describe("StatusPicker completion feedback", () => {
  it("shows a completion burst when an issue moves into Done", async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderWithI18n(<StatusPicker status="in_progress" onUpdate={onUpdate} />);

    await user.click(screen.getByRole("button", { name: "In Progress" }));
    await user.click(screen.getByRole("button", { name: "Done" }));

    expect(onUpdate).toHaveBeenCalledWith({ status: "done" });
    expect(document.querySelector("[data-completion-burst]")).toBeInTheDocument();

    animations.at(-1)?.dispatchEvent(new Event("finish"));
    expect(document.querySelector("[data-completion-burst]")).not.toBeInTheDocument();
  });

  it("keeps reduced-motion status changes still", async () => {
    reducedMotion.value = true;
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderWithI18n(<StatusPicker status="in_progress" onUpdate={onUpdate} />);

    await user.click(screen.getByRole("button", { name: "In Progress" }));
    await user.click(screen.getByRole("button", { name: "Done" }));

    expect(onUpdate).toHaveBeenCalledWith({ status: "done" });
    expect(document.querySelector("[data-completion-burst]")).not.toBeInTheDocument();
  });

  it("still completes the status change when animation setup fails", async () => {
    animate.mockImplementationOnce(() => {
      throw new Error("animation unavailable");
    });
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderWithI18n(<StatusPicker status="in_progress" onUpdate={onUpdate} />);

    await user.click(screen.getByRole("button", { name: "In Progress" }));
    await user.click(screen.getByRole("button", { name: "Done" }));

    expect(onUpdate).toHaveBeenCalledWith({ status: "done" });
    expect(document.querySelector("[data-completion-burst]")).not.toBeInTheDocument();
  });
});
