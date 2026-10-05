import { useParams } from "react-router-dom";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  PersonalWikiPageView,
  PersonalWikiRevisionView,
  WikiPageView,
  WorkspaceWikiRevisionView,
} from "@multica/views/wiki";

function useDesktopPersonalWikiPaths() {
  const workspacePaths = useWorkspacePaths();
  return {
    list: workspacePaths.personalWiki,
    page: workspacePaths.personalWikiPage,
    revision: workspacePaths.personalWikiRevision,
  };
}

export function WikiListPage() {
  const workspacePaths = useWorkspacePaths();
  return <WikiPageView personalWikiPath={workspacePaths.personalWiki()} />;
}

export function WikiDetailPage() {
  const { id } = useParams<{ id: string }>();
  const workspacePaths = useWorkspacePaths();
  return <WikiPageView pageId={id} personalWikiPath={workspacePaths.personalWiki()} />;
}

export function WikiRevisionPage() {
  const { revisionId = "" } = useParams<{ revisionId: string }>();
  return <WorkspaceWikiRevisionView revisionId={revisionId} />;
}

export function PersonalWikiListPage() {
  return <PersonalWikiPageView routePaths={useDesktopPersonalWikiPaths()} />;
}

export function PersonalWikiDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <PersonalWikiPageView pageId={id} routePaths={useDesktopPersonalWikiPaths()} />;
}

export function PersonalWikiRevisionPage() {
  const { revisionId = "" } = useParams<{ revisionId: string }>();
  const workspacePaths = useWorkspacePaths();
  return (
    <PersonalWikiRevisionView
      revisionId={revisionId}
      listPath={workspacePaths.personalWiki()}
    />
  );
}
