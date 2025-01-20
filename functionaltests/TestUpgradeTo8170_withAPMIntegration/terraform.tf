terraform {
  required_version = ">= 0.12.29"

  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.5.1"
    }
  }
}

locals {
  api_endpoints = {
    qa  = "https://public-api.qa.cld.elstc.co"
    pro = "https://api.elastic-cloud.com"
  }
}

provider "ec" {
  endpoint = local.api_endpoints[var.ec_target]
}
