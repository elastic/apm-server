terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">=4.27.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">=3.1.1"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">=4.0.1"
    }
    remote = {
      source  = "tenstad/remote"
      version = ">=0.1.0"
    }
  }
}
