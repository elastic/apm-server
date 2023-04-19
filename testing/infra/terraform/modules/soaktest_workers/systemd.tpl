[Unit]
Description=Workers to generate load for APM-Server soaktesting
ConditionPathExists=${apmsoak_executable_path}
After=network.target

[Service]
Type=simple
User=${remote_user}
Group=${remote_usergroup}
LimitNOFILE=1024

Restart=on-failure
RestartSec=5

ExecStart=${apmsoak_executable_path} \
  -server ${apm_server_url} \
  -secret-token ${apm_secret_token} \
  -event-rate ${apm_loadgen_event_rate} \
  -agents-replicas ${apm_loadgen_agents_replicas} \
  -rewrite-timestamps ${apm_loadgen_rewrite_timestamps} \
  -rewrite-ids ${apm_loadgen_rewrite_ids}

[Install]
WantedBy=multi-user.target

