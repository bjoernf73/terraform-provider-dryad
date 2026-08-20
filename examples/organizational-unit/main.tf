terraform {
  required_providers {
    dryad = {
      source = "bjoernf73/dryad"
    }
  }
}

provider "dryad" {
  transport       = "winrm"
  host            = var.host
  port            = var.port
  username        = var.username
  password        = var.password
  winrm_use_tls   = var.winrm_use_tls
  winrm_auth      = var.winrm_auth
  winrm_kerberos_realm = var.winrm_kerberos_realm
  powershell_path = "pwsh"
}

resource "dryad_organizational_unit" "example" {
  path        = "Contoso/Servers/Windows"
  description = "Windows server OU"
}
