// Plugin install/remove jobs. They run in the background and outlive the
// Plugins page, so what reacts to them lives outside it (PluginJobWatcher).

import type { Job } from './api';

export const PLUGIN_JOB_KINDS = new Set(['plugin_install', 'plugin_remove']);

export const isPluginJob = (j: Job) => PLUGIN_JOB_KINDS.has(j.kind);

// Jobs this browser started, so a failure is reported even after the user
// has navigated away from the Plugins page.
export const startedPluginJobs = new Set<string>();
