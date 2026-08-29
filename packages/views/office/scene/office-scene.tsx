import { useEffect, useRef } from "react";
import type {
  OfficeRendererStatus,
  OfficeSceneCommit,
  OfficeSceneHandle,
} from "./contracts";
import type { OfficeSubjectRef } from "@multica/core/office";

export interface OfficeSceneProps {
  readonly commit: OfficeSceneCommit;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly onStatus: (status: OfficeRendererStatus) => void;
  readonly className?: string;
}

export function OfficeScene({
  commit,
  onSelect,
  onStatus,
  className,
}: OfficeSceneProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const handleRef = useRef<OfficeSceneHandle | null>(null);
  const commitRef = useRef(commit);
  const onSelectRef = useRef(onSelect);
  const onStatusRef = useRef(onStatus);
  commitRef.current = commit;
  onSelectRef.current = onSelect;
  onStatusRef.current = onStatus;

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    let disposed = false;
    let pendingHandle: OfficeSceneHandle | null = null;

    void import("./create-office-scene")
      .then(({ createOfficeScene }) =>
        createOfficeScene({
          host,
          onSelect: (subject) => onSelectRef.current(subject),
          onStatus: (status) => onStatusRef.current(status),
        }),
      )
      .then((handle) => {
        pendingHandle = handle;
        if (disposed) {
          handle.destroy();
          return;
        }
        handleRef.current = handle;
        handle.reconcile(commitRef.current);
      })
      .catch(() => {
        if (!disposed) {
          onStatusRef.current({ kind: "fallback", reason: "unsupported" });
        }
      });

    return () => {
      disposed = true;
      const handle = handleRef.current ?? pendingHandle;
      handleRef.current = null;
      handle?.destroy();
    };
  }, []);

  useEffect(() => {
    handleRef.current?.reconcile(commit);
  }, [commit]);

  return (
    <div
      ref={hostRef}
      aria-hidden="true"
      className={className}
      data-office-scene=""
      style={{
        height: "100%",
        minHeight: 0,
        overflow: "hidden",
        touchAction: "none",
        width: "100%",
      }}
    />
  );
}
