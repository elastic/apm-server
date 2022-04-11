- module: beat
  xpack.enabled: true
  period: 10s
  hosts: ["http://CONTAINER_NAME:6791"]
  basepath: "/processes/apm-server-default"
