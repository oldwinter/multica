"use client";

import { useState } from "react";
import { BookOpenText, BrainCircuit, SlidersHorizontal } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { useT } from "../../i18n";
import { PageHeader } from "../../layout/page-header";
import { useOptionalNavigation } from "../../navigation";
import { TwinPanel } from "./twin-panel";
import { TwinActivationReadiness } from "./twin-activation-readiness";
import { TwinUsePanel } from "./twin-use-panel";
import type { TwinWorkspaceProps } from "./twin-workspace-types";
import {
  buildTwinWorkspaceTabPath,
  DEFAULT_TWIN_WORKSPACE_TAB,
  isTwinWorkspaceTab,
  parseTwinWorkspaceTab,
  TWIN_WORKSPACE_TAB_QUERY_KEY,
  type TwinWorkspaceTab,
} from "./twin-workspace-tabs";
import { WikiPanel } from "./wiki-panel";
import { ReadOnlyNotice, WorkspaceStaleState, WorkspaceState } from "./workspace-state";

export type { TwinDetailState, TwinViewState, TwinWorkspaceProps } from "./twin-workspace-types";

type TwinWorkspaceViewProps = TwinWorkspaceProps & { rootElement?: "main" | "div" };

export function TwinWorkspaceView({ rootElement = "main", ...props }: TwinWorkspaceViewProps) {
  const { t } = useT("twins");
  const navigation = useOptionalNavigation();
  const Root = rootElement;
  const [fallbackTab, setFallbackTab] = useState<TwinWorkspaceTab>(DEFAULT_TWIN_WORKSPACE_TAB);
  const tab = navigation
    ? parseTwinWorkspaceTab(navigation.searchParams.get(TWIN_WORKSPACE_TAB_QUERY_KEY))
    : fallbackTab;
  const navigateTab = (next: TwinWorkspaceTab) => {
    if (navigation) {
      navigation.replace(buildTwinWorkspaceTabPath(navigation, next));
    } else {
      setFallbackTab(next);
    }
  };
  return (
    <Root className="min-h-0 flex-1 overflow-y-auto bg-page-canvas" data-twin-copy data-twin-workspace>
      <PageHeader className="gap-2 bg-page-canvas">
        <BrainCircuit className="size-4 text-muted-foreground" aria-hidden="true" />
        <h1 className="min-w-0 truncate text-title font-medium text-foreground">{t(($) => $.page.title)}</h1>
      </PageHeader>
      <div
        className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 pt-6 pb-chat-launcher sm:px-6 lg:px-8"
        data-testid="twin-workspace-content"
      >
        <header className="min-w-0 space-y-2">
          <div className="flex items-center gap-2 text-caption text-muted-foreground">
            <BrainCircuit className="size-4" aria-hidden="true" />
            <span>{t(($) => $.page.eyebrow)}</span>
          </div>
          <p className="max-w-2xl text-body text-muted-foreground">{t(($) => $.page.description)}</p>
        </header>

        {props.actionError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-body text-destructive">{props.actionError}</div> : null}
        {!props.canManageWiki || !props.canManageTwin ? <ReadOnlyNotice /> : null}
        {props.state === "ready" && props.overviewStale ? <WorkspaceStaleState onRetry={props.onRetry} /> : null}

        {props.state === "ready" ? (
          <>
            <TwinActivationReadiness wsId={props.wsId} onNavigate={navigateTab} />
            <Tabs value={tab} onValueChange={(value) => isTwinWorkspaceTab(value) && navigateTab(value)} className="gap-5">
              <TabsList variant="line" className="w-full justify-start">
                <TabsTrigger value="wiki"><BookOpenText aria-hidden="true" />{t(($) => $.tabs.wiki)}</TabsTrigger>
                <TabsTrigger value="twin"><BrainCircuit aria-hidden="true" />{t(($) => $.tabs.twin)}</TabsTrigger>
                <TabsTrigger value="use"><SlidersHorizontal aria-hidden="true" />{t(($) => $.tabs.use)}</TabsTrigger>
              </TabsList>
              <TabsContent value="wiki"><WikiPanel {...props} /></TabsContent>
              <TabsContent value="twin"><TwinPanel {...props} /></TabsContent>
              <TabsContent value="use">
                <TwinUsePanel
                  wsId={props.wsId}
                  versions={props.twin.versions}
                  currentVersionId={props.twin.current_version?.id ?? ""}
                  canManage={props.canManageTwin}
                  onNavigate={navigateTab}
                />
              </TabsContent>
            </Tabs>
          </>
        ) : <WorkspaceState state={props.state} onRetry={props.onRetry} />}
      </div>
    </Root>
  );
}
