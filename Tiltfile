custom_build(
  'apm-server',
  'DOCKER_BUILDKIT=1 docker build -t $EXPECTED_REF -f packaging/docker/Dockerfile .',
  deps = ['.'],
  ignore = ['**/*_test.go', 'tools/**', 'systemtest/**', 'docs/**'],
)

# Build and install the APM integration package whenever source under
# "apmpackage" changes.
script_dir = os.path.join(config.main_dir, 'script')
run_with_go_ver = os.path.join(script_dir, 'run_with_go_ver')
local_resource(
  'apmpackage',
  cmd = [os.path.join(script_dir, 'run_with_go_ver'), 'go', 'run', './cmd/runapm -init'],
  dir = 'systemtest',
  deps = ['apmpackage'],
  resource_deps=['kibana'],
)

k8s_yaml(kustomize('testing/infra/k8s/overlays/local'))

k8s_kind('ApmServer', image_json_path='{.spec.image}')
k8s_kind('Kibana')
k8s_kind('Elasticsearch')

k8s_resource('elastic-operator', objects=['eck-trial-license:Secret:elastic-system'])
k8s_resource('apm-server', port_forwards=8200)
k8s_resource('kibana', port_forwards=5601)
k8s_resource('elasticsearch', port_forwards=9200, objects=['elasticsearch-admin:Secret:default'])

# Delete ECK entirely on `tilt down`, to ensure `tilt up` starts from a clean slate.
#
# Without this, ECK refuses to recreate the trial license. That's expected
# for actual trial use, but we rely an Enterprise feature for local development:
# installing the integration package by uploading to Fleet.
if config.tilt_subcommand == "down":
  print(local("kubectl delete --ignore-not-found namespace/elastic-system"))

# Add a button for sending trace events and metrics to APM Server.
load('ext://uibutton', 'cmd_button')
cmd_button(
  'apm-server:sendotlp',
  argv=['sh', '-c', 'cd systemtest && %s go run ./cmd/sendotlp' % run_with_go_ver],
  resource='apm-server',
  icon_name='input',
  text='Send OTLP data',
)
