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

ExecStart=${apmsoak_executable_path} run \
  --server-url ${apm_server_url} \
  --secret-token ${apm_secret_token} \
  --file ${scenarios_yml}

[Install]
WantedBy=multi-user.target

