"use client";

import { useState } from "react";
import { Brain, FileText, ShieldCheck } from "lucide-react";
import type { TwinOverview } from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { buttonVariants } from "@multica/ui/components/ui/button";
import { Separator as UiSeparator } from "@multica/ui/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { AppLink } from "../../navigation";
import { EvidenceSnapshot, TopicList } from "./twin-workspace-lists";
import { ReviewPath, StatePanel, StatusBanner, SummaryMetrics } from "./twin-workspace-status";
import type { TwinCopy, TwinViewState } from "./twin-workspace-types";

export type { TwinCopy, TwinViewState } from "./twin-workspace-types";

interface TwinWorkspaceViewProps {
  data?: TwinOverview;
  state?: TwinViewState;
  copy: TwinCopy;
  links: {
    issues: string;
    agents: string;
  };
  onRetry: () => void;
}

type TabValue = "overview" | "evidence" | "topics";

export function TwinWorkspaceView({
  data,
  state = "ready",
  copy,
  links,
  onRetry,
}: TwinWorkspaceViewProps) {
  const [activeTab, setActiveTab] = useState<TabValue>("overview");
  const resolvedData = state === "ready" ? data : undefined;

  return (
    <main className="pe-chat-launcher min-h-0 flex-1 overflow-y-auto bg-page-canvas" data-testid="twin-workspace">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-5 sm:px-6 lg:px-8 lg:py-7" data-twin-copy>
        <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
              <Brain className="size-3.5" aria-hidden="true" />
              <span>{copy.eyebrow}</span>
              {resolvedData ? <Badge variant="outline">{copy.previewBadge}</Badge> : null}
            </div>
            <h1 className="text-display-sm font-medium text-foreground">{copy.title}</h1>
            <p className="max-w-2xl text-body text-muted-foreground">{copy.description}</p>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            <AppLink href={links.issues} className={buttonVariants({ variant: "outline" })}>
              <FileText data-icon="inline-start" />
              {copy.actions.openIssues}
            </AppLink>
            <AppLink href={links.agents} className={buttonVariants({ variant: "ghost" })}>
              <ShieldCheck data-icon="inline-start" />
              {copy.actions.openAgents}
            </AppLink>
          </div>
        </header>

        {resolvedData ? (
          <>
            <StatusBanner data={resolvedData} copy={copy} onReview={() => setActiveTab("evidence")} />
            <SummaryMetrics data={resolvedData} copy={copy} />
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as TabValue)} className="gap-4">
              <TabsList variant="line" className="w-full flex-wrap justify-start max-sm:h-auto max-sm:gap-x-1 max-sm:gap-y-1">
                <TabsTrigger value="overview" className="max-sm:min-h-8 max-sm:flex-none max-sm:whitespace-nowrap max-sm:leading-snug">
                  {copy.tabs.overview}
                </TabsTrigger>
                <TabsTrigger value="evidence" className="max-sm:min-h-8 max-sm:flex-none max-sm:whitespace-nowrap max-sm:leading-snug">
                  {copy.tabs.evidence}
                </TabsTrigger>
                <TabsTrigger value="topics" className="max-sm:min-h-8 max-sm:flex-none max-sm:whitespace-nowrap max-sm:leading-snug">
                  {copy.tabs.topics}
                </TabsTrigger>
              </TabsList>
              <TabsContent value="overview" className="grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(18rem,0.9fr)]">
                <ReviewPath data={resolvedData} copy={copy} />
                <TopicList data={resolvedData} copy={copy} links={links} />
              </TabsContent>
              <TabsContent value="evidence">
                <EvidenceSnapshot data={resolvedData} copy={copy} />
              </TabsContent>
              <TabsContent value="topics">
                <TopicList data={resolvedData} copy={copy} links={links} />
              </TabsContent>
            </Tabs>
            <UiSeparator className="bg-border/70" />
            <p className="font-mono text-caption text-muted-foreground">{resolvedData.reviewDigest}</p>
          </>
        ) : (
          <StatePanel
            state={state === "ready" ? "empty" : state}
            copy={copy}
            onRetry={onRetry}
            emptyHref={links.issues}
          />
        )}
      </div>
    </main>
  );
}
