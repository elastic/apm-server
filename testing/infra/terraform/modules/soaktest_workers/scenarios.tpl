scenarios:
  steady:
    - event_rate: ${apm_loadgen_event_rate}
      agents_replicas: ${apm_loadgen_agents_replicas}
      rewrite_timestamps: ${apm_loadgen_rewrite_timestamps}
      rewrite_ids: ${apm_loadgen_rewrite_ids}
