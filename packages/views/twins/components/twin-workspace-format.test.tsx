// @vitest-environment jsdom

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { formatTwinTimestamp, TwinTimestamp } from "./twin-workspace-format";

describe("formatTwinTimestamp", () => {
  it("formats a valid ISO timestamp with medium date and short time", () => {
    const formatted = formatTwinTimestamp("2026-08-02T14:30:00.000Z");
    // Locale-dependent, but must not be the raw ISO string and must include year.
    expect(formatted).not.toBe("2026-08-02T14:30:00.000Z");
    expect(formatted).toMatch(/2026/);
    expect(Number.isNaN(Date.parse(formatted)) || formatted.includes("2026")).toBe(true);
  });

  it("passes through non-timestamp display strings unchanged", () => {
    expect(formatTwinTimestamp("Today")).toBe("Today");
    expect(formatTwinTimestamp("Yesterday")).toBe("Yesterday");
    expect(formatTwinTimestamp("not-a-date")).toBe("not-a-date");
    expect(formatTwinTimestamp("")).toBe("");
  });
});

describe("TwinTimestamp", () => {
  it("renders a time element for ISO values and keeps the original dateTime", async () => {
    const value = "2026-08-02T14:30:00.000Z";
    render(<TwinTimestamp value={value} />);

    const time = screen.getByText((_, element) => element?.tagName === "TIME") as HTMLTimeElement;
    expect(time).toHaveAttribute("dateTime", value);
    expect(time.textContent).not.toBe(value);
    await waitFor(() => {
      expect(time.textContent).toMatch(/2026/);
    });
  });

  it("renders plain text for non-timestamp values", () => {
    render(<TwinTimestamp value="Yesterday" />);
    expect(screen.getByText("Yesterday").tagName).toBe("SPAN");
    expect(screen.queryByRole("time")).toBeNull();
  });
});
