import { useParams } from "react-router-dom";
import { WikiPageView } from "@multica/views/wiki";

export function WikiListPage() {
  return <WikiPageView />;
}

export function WikiDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <WikiPageView pageId={id} />;
}
