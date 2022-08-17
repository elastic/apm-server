# Cloud-first testing

It is possible for Elastic employees to create an Elastic Cloud deployment with a locally
built APM Server binary and APM integration package (i.e. with modifications in the current
working tree), by pushing images to an internal Docker repository. The images will be based
off the SNAPSHOT images referenced in docker-compose.yml.

Running `make` in this directory will build and push the images. You can then use Terraform
to create the deployment with `EC_API_KEY=your_api_key terraform apply -auto-approve`.

The custom images are tagged with the current user (i.e. `$USER`) and a timestamp. The
timestamp is included to force a new Docker image to be used, which enables pushing new
apm-server binaries without recreating the deployment. Kibana only installs the integration
package when it first starts up, so any changes to the package will be disregarded when
updating an existing deployment.
