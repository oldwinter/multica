import { useCallback, useEffect, useRef, useState } from "react";
import type {
  OfficeInspector,
  OfficeSnapshot,
  OfficeSubjectRef,
} from "@multica/core/office";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from "@multica/ui/components/ui/sheet";
import { useT } from "../i18n";
import { OfficeInspectorPanel } from "./office-inspector";
import {
  OfficeRoster,
  officeRosterTabForSubject,
  officeSubjectKey,
  type OfficeRosterTab,
} from "./office-roster";

const INITIAL_ROVING: Record<OfficeRosterTab, OfficeSubjectRef | null> = {
  agents: null,
  squads: null,
  issues: null,
};

export function OfficeRail({
  snapshot,
  inspector,
  selected,
  narrow,
  onSelect,
}: {
  readonly snapshot: OfficeSnapshot;
  readonly inspector: OfficeInspector;
  readonly selected: OfficeSubjectRef | null;
  readonly narrow: boolean;
  readonly onSelect: (subject: OfficeSubjectRef | null) => void;
}) {
  const { t } = useT("office");
  const [activeTab, setActiveTab] = useState<OfficeRosterTab>("agents");
  const [rovingByTab, setRovingByTab] =
    useState<Record<OfficeRosterTab, OfficeSubjectRef | null>>(INITIAL_ROVING);
  const rowRefs = useRef(new Map<string, HTMLButtonElement>());
  const originRef = useRef<OfficeSubjectRef | null>(null);
  const pendingFocusRef = useRef<OfficeSubjectRef | null>(null);

  useEffect(() => {
    if (!selected) return;
    const tab = officeRosterTabForSubject(selected);
    setActiveTab(tab);
    setRovingByTab((current) =>
      current[tab] &&
      officeSubjectKey(current[tab]) === officeSubjectKey(selected)
        ? current
        : { ...current, [tab]: selected },
    );
    if (!originRef.current) originRef.current = selected;
  }, [selected]);

  useEffect(() => {
    if (selected || !pendingFocusRef.current) return;
    const subject = pendingFocusRef.current;
    pendingFocusRef.current = null;
    rowRefs.current.get(officeSubjectKey(subject))?.focus();
    originRef.current = null;
  }, [inspector.kind, selected]);

  const handleSelect = (subject: OfficeSubjectRef) => {
    const tab = officeRosterTabForSubject(subject);
    originRef.current = subject;
    setActiveTab(tab);
    setRovingByTab((current) => ({ ...current, [tab]: subject }));
    onSelect(subject);
  };

  const handleBack = useCallback(() => {
    pendingFocusRef.current = originRef.current ?? selected;
    onSelect(null);
  }, [onSelect, selected]);

  useEffect(() => {
    if (inspector.kind === "closed") return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.key !== "Escape") return;
      event.preventDefault();
      handleBack();
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [handleBack, inspector.kind]);

  const roster = (
    <OfficeRoster
      snapshot={snapshot}
      activeTab={activeTab}
      onActiveTabChange={setActiveTab}
      onSelect={handleSelect}
      registerRow={(subject, element) => {
        const key = officeSubjectKey(subject);
        if (element) rowRefs.current.set(key, element);
        else rowRefs.current.delete(key);
      }}
      rovingSubject={rovingByTab[activeTab]}
      onRovingSubjectChange={(subject) => {
        const tab = officeRosterTabForSubject(subject);
        setRovingByTab((current) =>
          current[tab] &&
          officeSubjectKey(current[tab]) === officeSubjectKey(subject)
            ? current
            : { ...current, [tab]: subject },
        );
      }}
    />
  );

  if (narrow) {
    const sheetOpen = inspector.kind !== "closed";
    return (
      <>
        {roster}
        <Sheet
          open={sheetOpen}
          onOpenChange={(open) => {
            if (!open && sheetOpen) handleBack();
          }}
        >
          <SheetContent
            side="right"
            showCloseButton={false}
            className="w-full max-w-[min(92vw,360px)] gap-0 p-0"
          >
            <SheetTitle className="sr-only">
              {t(($) => $.roster.title)}
            </SheetTitle>
            <SheetDescription className="sr-only">
              {t(($) => $.inspector.back)}
            </SheetDescription>
            {inspector.kind !== "closed" ? (
              <OfficeInspectorPanel
                inspector={inspector}
                snapshot={snapshot}
                onBack={handleBack}
              />
            ) : null}
          </SheetContent>
        </Sheet>
      </>
    );
  }

  return inspector.kind === "closed" ? (
    roster
  ) : (
    <OfficeInspectorPanel
      inspector={inspector}
      snapshot={snapshot}
      onBack={handleBack}
    />
  );
}
