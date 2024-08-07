<!-- BEGIN_TF_DOCS -->
## Default tags module

This modules exports the default tags / labels to use on Cloud Service Providers based on the Elastic tagging policy.

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_project"></a> [project](#input\_project) | The value to use for the project tag/label | `any` | n/a | yes |
| <a name="input_build"></a> [build](#input\_build) | The value to use for the build tag/label, normally related to the CICD executions. | `any` | `unknown` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_labels"></a> [labels](#output\_labels) | Labels for CSP resources |
| <a name="output_tags"></a> [tags](#output\_tags) | Tags for CSP resources |
<!-- END_TF_DOCS -->