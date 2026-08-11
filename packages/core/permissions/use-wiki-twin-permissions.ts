"use client";

import { useCurrentMember } from "./use-current-member";
import { canManageWikiTwin } from "./rules";
import type { Decision } from "./types";

export function useWikiTwinPermissions(
  wsId: string,
  serverCanManage: boolean,
): {
  canManage: Decision;
  canMutate: boolean;
  isLoading: boolean;
} {
  const { userId, role, isLoading } = useCurrentMember(wsId);
  const canManage = canManageWikiTwin({ userId, role });
  return {
    canManage,
    canMutate: serverCanManage === true && canManage.allowed,
    isLoading,
  };
}
