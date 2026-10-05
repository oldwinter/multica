"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@multica/core/auth";
import { paths } from "@multica/core/paths";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";

export default function PersonalWikiLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const isLoading = useAuthStore((state) => state.isLoading);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace(`${paths.login()}?next=${encodeURIComponent(paths.personalWiki())}`);
    }
  }, [isLoading, router, user]);

  useEffect(() => {
    if (user && user.onboarded_at == null) router.replace(paths.onboarding());
  }, [router, user]);

  if (isLoading || !user || user.onboarded_at == null) {
    return (
      <div className="flex h-svh items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return <div className="flex h-svh min-w-0 flex-col">{children}</div>;
}
