"use client";

import { useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import type { IssueStatus, UpdateIssueRequest } from "@multica/core/types";
import { ALL_STATUSES, STATUS_CONFIG } from "@multica/core/issues/config";
import {
  UI_EASE_OUT,
  UI_MOTION_DURATION,
} from "@multica/ui/lib/motion";
import { StatusIcon } from "../status-icon";
import { PropertyPicker, PickerItem } from "./property-picker";
import { useT } from "../../../i18n";

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

function showCompletionBurst(origin: CompletionBurstOrigin) {
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
  const { t } = useT("issues");

  return (
    <span ref={triggerRef} className="relative inline-flex min-w-0 max-w-full">
      <PropertyPicker
        open={open}
        onOpenChange={setOpen}
        width="w-44"
        align={align}
        triggerRender={triggerRender}
        trigger={
          customTrigger ??
          (status != null ? (
            <>
              <StatusIcon status={status} className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate">{t(($) => $.status[status])}</span>
            </>
          ) : null)
        }
      >
        {ALL_STATUSES.map((s) => {
          const c = STATUS_CONFIG[s];
          return (
            <PickerItem
              key={s}
              selected={s === status}
              hoverClassName={c.hoverBg}
              onClick={() => {
                let completionOrigin: CompletionBurstOrigin | null = null;
                if (s === "done" && status !== "done" && !shouldReduceMotion) {
                  const bounds = triggerRef.current?.getBoundingClientRect();
                  if (bounds) {
                    completionOrigin = {
                      x: bounds.left + bounds.width / 2,
                      y: bounds.top + bounds.height / 2,
                    };
                  }
                }
                onUpdate({ status: s });
                setOpen(false);
                if (completionOrigin) showCompletionBurst(completionOrigin);
              }}
            >
              <StatusIcon status={s} className="h-3.5 w-3.5" />
              <span>{t(($) => $.status[s])}</span>
            </PickerItem>
          );
        })}
      </PropertyPicker>
    </span>
  );
}
