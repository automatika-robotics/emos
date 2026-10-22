<script lang="ts">
  // Mounted once in the app shell. When a plugin job ends, refreshes what the
  // dashboard shows about the robot on whichever page is open, and reports a
  // failed job this browser started.
  import { useQueryClient } from '@tanstack/svelte-query';
  import { usePluginJobsWatch, keys } from '$lib/queries';
  import { isPluginJob, startedPluginJobs } from '$lib/pluginJobs';
  import { confirm as confirmDialog } from '$lib/dialog';

  const qc = useQueryClient();
  const jobs = usePluginJobsWatch();

  // Plugin jobs last seen running. A job that had already ended when the
  // dashboard loaded is in neither set and is ignored.
  const running = new Set<string>();

  $effect(() => {
    for (const j of $jobs.data ?? []) {
      if (!isPluginJob(j)) continue;
      if (j.status === 'running') {
        running.add(j.id);
        continue;
      }
      const started = startedPluginJobs.delete(j.id);
      if (!running.delete(j.id) && !started) continue;

      qc.invalidateQueries({ queryKey: keys.pluginsInstalled });
      qc.invalidateQueries({ queryKey: keys.robot });
      if (j.status === 'failed' && started) {
        const action = j.kind === 'plugin_install' ? 'Installing' : 'Removing';
        confirmDialog({
          title: `${action} ${j.target} failed`,
          message: j.error || j.message,
          confirmLabel: 'OK',
          cancelLabel: 'Dismiss',
        });
      }
    }
  });
</script>
