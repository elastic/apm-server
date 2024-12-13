terraform {
  required_version = ">= 0.12.29"

  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.12.2"
    }
  }
}

provider "ec" {
  endpoint = "https://public-api.qa.cld.elstc.co"
}
