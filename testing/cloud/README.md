# Cloud-first testing

It is possible for Elastic employees to create an Elastic Cloud deployment with a locally
built APM Server binary and APM integration package (i.e. with modifications in the current
working tree), by pushing images to an internal Docker repository. The images will be based
off the SNAPSHOT images referenced in docker-compose.yml.

Running `make` in this directory will build and push the images. You can then use Terraform
to create the deployment with `EC_API_KEY=your_api_key make apply`.
NOTE: ESS API Keys can be created via https://cloud.elastic.co/deployment-features/keys.

The custom images are tagged with the current user (i.e. `$USER`) and a timestamp. The
timestamp is included to force a new Docker image to be used, which enables pushing new
apm-server binaries without recreating the deployment. Kibana only installs the integration
package when it first starts up, so any changes to the package will be disregarded when
updating an existing deployment.

After finishing the testing, run `make destroy` for deleting the clusters. 

## Building and deploying a custom Elastic Agent image

For testing a non-released Elastic Agent image, the cloud base image needs to be configured via `ELASTIC_AGENT_DOCKER_IMAGE` and `ELASTIC_AGENT_IMAGE_TAG`. 
```
ELASTIC_AGENT_DOCKER_IMAGE=docker.elastic.co/observability-ci/elastic-agent-cloud ELASTIC_AGENT_IMAGE_TAG=a8b36d05919c40385fe5a28d30225a112626d48b make elastic_agent_docker_image docker_image.auto.tfvars
EC_API_KEY=<your-api-key> make apply
```
NOTE: Before running the above command, ensure that the file `docker_image.auto.tfvars` does not exist, as it won't be overwritten.

Take a look at the `variables.tf` for specifying sizes, version, etc. 

Example - testing local changes for Elastic Agent:
First, build the elastic agent docker image from your local branch and push it to the internal docker registry, e.g. `docker.elastic.co/observability-ci/elastic-agent:8.6.1-8f6887b9-SNAPSHOT-simitt`.
In this case, the command to build the docker image and create the deployment would look like this: 
```
ELASTIC_AGENT_DOCKER_IMAGE=docker.elastic.co/observability-ci/elastic-agent-cloud ELASTIC_AGENT_IMAGE_TAG=a8b36d05919c40385fe5a28d30225a112626d48b make elastic_agent_docker_image docker_image.auto.tfvars
EC_API_KEY=<your-api-key> make apply
```

