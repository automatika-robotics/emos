// TanStack Query glue. Centralising hook keys here makes invalidation
// (e.g. after a recipe pull finishes) one-liner-clean from any component.

import { createQuery, createMutation, useQueryClient, type CreateQueryResult } from '@tanstack/svelte-query';
import { api, ApiException } from './api';

export const keys = {
  health: ['health'] as const,
  info: ['info'] as const,
  capabilities: ['capabilities'] as const,
  connectivity: ['connectivity'] as const,
  authMe: ['authMe'] as const,
  robot: ['robot'] as const,
  recipesLocal: ['recipes', 'local'] as const,
  recipesRemote: ['recipes', 'remote'] as const,
  recipeDetail: (name: string) => ['recipes', 'detail', name] as const,
  runs: ['runs'] as const,
  run: (id: string) => ['runs', id] as const,
  jobs: ['jobs'] as const,
  job: (id: string) => ['jobs', id] as const,
};

// info refreshes every 5s — the dashboard cards are driven from this.
export const useInfo = () =>
  createQuery({ queryKey: keys.info, queryFn: api.info, refetchInterval: 5000, staleTime: 2000 });

export const useCapabilities = () =>
  createQuery({ queryKey: keys.capabilities, queryFn: api.capabilities, staleTime: 30_000 });

export const useConnectivity = () =>
  createQuery({
    queryKey: keys.connectivity,
    queryFn: () => api.connectivity(false),
    refetchInterval: 10_000,
    staleTime: 5000,
  });

export const useRobot = () =>
  createQuery({
    queryKey: keys.robot,
    // 404 is not an error in our model — the dashboard renders a generic device
    // card if /robot returns nothing. Treat 404 as null.
    queryFn: async () => {
      try {
        return await api.robot();
      } catch (err) {
        if (err instanceof ApiException && err.status === 404) return null;
        throw err;
      }
    },
    staleTime: 60_000,
  });

export const useRecipesLocal = () =>
  createQuery({ queryKey: keys.recipesLocal, queryFn: api.recipesLocal, staleTime: 5000 });

export const useRecipesRemote = () =>
  createQuery({
    queryKey: keys.recipesRemote,
    queryFn: api.recipesRemote,
    retry: (count, err) => {
      // Don't retry the offline state — it's a definitive answer.
      if (err instanceof ApiException && err.code === 'offline') return false;
      return count < 1;
    },
    staleTime: 30_000,
  });

export const useRecipeDetail = (name: string) =>
  createQuery({
    queryKey: keys.recipeDetail(name),
    queryFn: () => api.recipeDetail(name),
    enabled: !!name,
  });

export const useRuns = () =>
  createQuery({ queryKey: keys.runs, queryFn: api.runs, refetchInterval: 3000 });

export const useRun = (id: string) =>
  createQuery({
    queryKey: keys.run(id),
    queryFn: () => api.runGet(id),
    enabled: !!id,
    // Poll while the run could still transition. Once it's a terminal
    // state the query stops refetching automatically.
    refetchInterval: (q) => {
      const s = (q.state.data as any)?.status;
      return s === 'preparing' || s === 'running' ? 1500 : false;
    },
  });

export const useJobs = () =>
  // 2s cadence — faster than runs because pull jobs are short-lived (often
  // <10s) and we want the card to reflect failure/success without lag.
  createQuery({ queryKey: keys.jobs, queryFn: api.jobs, refetchInterval: 2000 });

// Mutations -----------------------------------------------------------------

export function useStartRun() {
  const qc = useQueryClient();
  return createMutation({
    mutationFn: (vars: { recipe: string; rmw?: string; skip_sensor_check?: boolean }) =>
      api.runStart(vars.recipe, { rmw: vars.rmw, skip_sensor_check: vars.skip_sensor_check }),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.runs }),
  });
}

export function useCancelRun() {
  const qc = useQueryClient();
  return createMutation({
    mutationFn: (id: string) => api.runCancel(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.runs }),
  });
}

export function usePullRecipe() {
  const qc = useQueryClient();
  return createMutation({
    mutationFn: (name: string) => api.recipePull(name),
    onSuccess: () => {
      // The mutation only confirms the job was REGISTERED (HTTP 202).
      // The download finishes later, async, on the backend. The actual
      // "now appears in Installed" trigger lives in Recipes.svelte,
      // which watches the polled jobs list and invalidates recipesLocal
      // when it sees a recipe_pull transition to "finished".
      qc.invalidateQueries({ queryKey: keys.jobs });
    },
  });
}

export function useDeleteRecipe() {
  const qc = useQueryClient();
  return createMutation({
    mutationFn: (name: string) => api.recipeDelete(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.recipesLocal }),
  });
}

export type Query<T> = CreateQueryResult<T, Error>;
