terraform {
  required_version = ">= 1.11.0"

  required_providers {
    powerdns = {
      source  = "mmianl/powerdns"
      version = "999.0.0"
    }
  }
}
