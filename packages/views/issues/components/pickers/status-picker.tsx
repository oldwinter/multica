"use client";

import { useMemo, useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import type { IssueStatus, UpdateIssueRequest } from "@multica/core/types";
import { STATUS_CONFIG } from "@multica/core/issues/config";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { useWorkspaceId } from "@multica/core/hooks";
import { UI_EASE_OUT, UI_MOTION_DURATION } from "@multica/ui/lib/motion";
import { StatusIcon } from "../status-icon";
import { PropertyPicker, PickerItem } from "./property-picker";
import { useT } from "../../../i18n";
import { useStatusLabel } from "../../utils/status-label";
import { useStatusOptions } from "../../utils/status-options";

/** Above this many options the flat list stops being scannable. */
const SEARCH_THRESHOLD = 9;

const COMPLETION_PARTICLES = [
  { x: 0, y: -30 },
  { x: 21, y: -21 },
  { x: 30, y: 0 },
  { x: 21, y: 21 },
  { x: 0, y: 30 },
  { x: -21, y: 21 },
  { x: -30, y: 0 },
  { x: -21, y: -21 },
] as const;

type CompletionBurstOrigin = {
  readonly x: number;
  readonly y: number;
};

function showCompletionBurst(origin: CompletionBurstOrigin): void {
  const duration = UI_MOTION_DURATION.standard * 3 * 1000;
  const easing = `cubic-bezier(${UI_EASE_OUT.join(",")})`;
  const burst = document.createElement("span");
  burst.ariaHidden = "true";
  burst.dataset.completionBurst = "";
  burst.className = "pointer-events-none fixed z-[60]";
  burst.style.left = `${origin.x}px`;
  burst.style.top = `${origin.y}px`;

  document.body.append(burst);

  try {
    for (const particle of COMPLETION_PARTICLES) {
      const dot = document.createElement("span");
      dot.className =
        "absolute -left-[3px] -top-[3px] size-1.5 rounded-full bg-success";
      burst.append(dot);
      dot.animate(
        [
          { opacity: 0, transform: "translate(0, 0) scale(0.5)" },
          {
            opacity: 1,
            transform: `translate(${particle.x * 0.55}px, ${particle.y * 0.55}px) scale(1)`,
            offset: 0.45,
          },
          {
            opacity: 0,
            transform: `translate(${particle.x}px, ${particle.y}px) scale(0.5)`,
          },
        ],
        { duration, easing, fill: "forwards" },
      );
    }

    const ring = document.createElement("span");
    ring.className =
      "absolute -left-3.5 -top-3.5 size-7 rounded-full border-2 border-success/70";
    burst.append(ring);
    const ringAnimation = ring.animate(
      [
        { opacity: 0.8, transform: "scale(0.3)" },
        { opacity: 0, transform: "scale(1.8)" },
      ],
      { duration, easing, fill: "forwards" },
    );
    ringAnimation.addEventListener("finish", () => burst.remove(), { once: true });
  } catch {
    burst.remove();
  }
}

export function StatusPicker({
  status,
  onUpdate,
  trigger: customTrigger,
  triggerRender,
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  align,
}: {
  /**
   * The currently-selected status, used to check the matching row. `null`
   * means "no single current value" (e.g. a batch selection spanning several
   * statuses) — no row is checked. Single-issue callers always pass a concrete
   * status.
   */
  status: IssueStatus | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  align?: "start" | "center" | "end";
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const triggerRef = useRef<HTMLSpanElement>(null);
  const shouldReduceMotion = useReducedMotion() ?? false;
  const open = controlledOpen ?? internalOpen;
  const setOpen = controlledOnOpenChange ?? setInternalOpen;
  const [query, setQuery] = useState("");
  const { t } = useT("issues");
  // Every StatusPicker call site lives inside the workspace shell (issue
  // detail, table, board batch toolbar, create-issue modal), so the provider
  // is guaranteed here.
  const wsId = useWorkspaceId();
  const { categoryOf, colorOf } = useIssueStatuses(wsId);
  const labelOf = useStatusLabel(wsId);

  /**
   * Offerable statuses as one flat list, in canonical category order.
   *
   * Archived statuses are excluded: archiving retires a status from future
   * assignment while leaving the issues already on it untouched. Falls back to
   * the 7 built-ins until the catalog lands, so a cold render offers exactly
   * what it always did instead of an empty popover. (MUL-6243)
   */
  const allOptions = useStatusOptions(wsId);

  const options = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return allOptions;
    return allOptions.filter((o) => o.label.toLowerCase().includes(q));
  }, [allOptions, query]);

  const searchable = allOptions.length > SEARCH_THRESHOLD;

  return (
    <span ref={triggerRef} className="relative inline-flex min-w-0 max-w-full">
      <PropertyPicker
        open={open}
        onOpenChange={(v) => {
          if (!v) setQuery("");
          setOpen(v);
        }}
        width="w-52"
        align={align}
        triggerRender={triggerRender}
        searchable={searchable}
        searchPlaceholder={t(($) => $.filters.search_status)}
        onSearchChange={setQuery}
        trigger={
          customTrigger ??
          (status != null ? (
            <>
              <StatusIcon
                status={status}
                category={categoryOf(status)}
                color={colorOf(status)}
                className="h-3.5 w-3.5 shrink-0"
              />
              <span className="truncate">{labelOf(status)}</span>
            </>
          ) : null)
        }
      >
        {options.map((option) => (
          <PickerItem
            key={option.key}
            selected={option.key === status}
            hoverClassName={STATUS_CONFIG[option.category].hoverBg}
            onClick={() => {
              let completionOrigin: CompletionBurstOrigin | null = null;
              const currentCategory = status == null ? null : categoryOf(status);
              if (
                option.category === "done" &&
                currentCategory !== "done" &&
                !shouldReduceMotion
              ) {
                const bounds = triggerRef.current?.getBoundingClientRect();
                if (bounds) {
                  completionOrigin = {
                    x: bounds.left + bounds.width / 2,
                    y: bounds.top + bounds.height / 2,
                  };
                }
              }
              onUpdate({ status: option.key });
              setOpen(false);
              setQuery("");
              if (completionOrigin) showCompletionBurst(completionOrigin);
            }}
          >
            <StatusIcon
              status={option.key}
              category={option.category}
              color={option.color}
              className="h-3.5 w-3.5"
            />
            <span className="truncate">{option.label}</span>
          </PickerItem>
        ))}
      </PropertyPicker>
    </span>
  );
}
