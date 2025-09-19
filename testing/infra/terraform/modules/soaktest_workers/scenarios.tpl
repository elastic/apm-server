scenarios:
  steady:
    - event-rate: ${apm_loadgen_event_rate}
      agents-replicas: ${apm_loadgen_agents_replicas}
      rewrite-timestamps: ${apm_loadgen_rewrite_timestamps}
      rewrite-ids: ${apm_loadgen_rewrite_ids}
