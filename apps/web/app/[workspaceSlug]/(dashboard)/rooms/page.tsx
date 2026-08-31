"use client";

import { RoomsPage } from "@multica/views/rooms";
import { ErrorBoundary } from "@multica/ui/components/common/error-boundary";

export default function Page() {
  return (
    <ErrorBoundary>
      <RoomsPage rootElement="div" />
    </ErrorBoundary>
  );
}
