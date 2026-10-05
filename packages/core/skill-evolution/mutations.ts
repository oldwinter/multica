import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { skillEvolutionKeys } from "./queries";
import type {
  ConfigureSkillEvolutionInput,
  DecideSkillEvolutionProposalInput,
  ForkSkillForEvolutionInput,
  RollbackSkillEvolutionReleaseInput,
  SkillEvolutionIdempotencyInput,
} from "./types";

export function useConfigureSkillEvolution(wsId: string, skillId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ConfigureSkillEvolutionInput) =>
      api.configureSkillEvolution(skillId, input),
    onSettled: () => queryClient.invalidateQueries({
      queryKey: skillEvolutionKeys.overview(wsId, skillId),
      exact: true,
    }),
  });
}

export function usePauseSkillEvolution(wsId: string, skillId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SkillEvolutionIdempotencyInput) =>
      api.pauseSkillEvolution(skillId, input),
    onSettled: () => queryClient.invalidateQueries({
      queryKey: skillEvolutionKeys.overview(wsId, skillId),
      exact: true,
    }),
  });
}

export function useRequestSkillEvolutionProposal(wsId: string, skillId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SkillEvolutionIdempotencyInput) =>
      api.requestSkillEvolutionProposal(skillId, input),
    onSettled: () => queryClient.invalidateQueries({
      queryKey: skillEvolutionKeys.overview(wsId, skillId),
      exact: true,
    }),
  });
}

export function useRejectSkillEvolutionProposal(
  wsId: string,
  skillId: string,
  proposalId: string,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DecideSkillEvolutionProposalInput) =>
      api.rejectSkillEvolutionProposal(proposalId, input),
    onSettled: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: skillEvolutionKeys.overview(wsId, skillId),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: skillEvolutionKeys.proposal(wsId, proposalId),
          exact: true,
        }),
      ]);
    },
  });
}

export function usePublishSkillEvolutionProposal(
  wsId: string,
  skillId: string,
  proposalId: string,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DecideSkillEvolutionProposalInput) =>
      api.publishSkillEvolutionProposal(proposalId, input),
    onSettled: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: skillEvolutionKeys.overview(wsId, skillId),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: skillEvolutionKeys.proposal(wsId, proposalId),
          exact: true,
        }),
      ]);
    },
  });
}

export function useRollbackSkillEvolutionRelease(
  wsId: string,
  skillId: string,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RollbackSkillEvolutionReleaseInput) =>
      api.rollbackSkillEvolutionRelease(skillId, input.releaseId, {
        idempotencyKey: input.idempotencyKey,
      }),
    onSettled: () => queryClient.invalidateQueries({
      queryKey: skillEvolutionKeys.overview(wsId, skillId),
      exact: true,
    }),
  });
}

export function useForkSkillForEvolution(wsId: string, sourceSkillId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ForkSkillForEvolutionInput) =>
      api.forkSkillForEvolution(sourceSkillId, input),
    onSettled: () => queryClient.invalidateQueries({
      queryKey: skillEvolutionKeys.overview(wsId, sourceSkillId),
      exact: true,
    }),
  });
}
