# terraform-provider-dryad

Initial scaffold for a Terraform provider that manages Active Directory through remote PowerShell execution on Windows hosts.

## Current scope

- Terraform Provider Plugin Framework
- WinRM transport
  - basic
  - NTLM
  - Kerberos
- SSH transport
  - password auth
  - private key auth
- `dryad_organizational_unit` resource

## Current resource model

The first resource intentionally focuses on the `dry.module.ad` OU behavior:

- input path is a slash-delimited path relative to the domain root
- missing parent OUs are created on demand
- description is reconciled on the leaf OU
- resource ID is the OU distinguished name

## Provider examples

### WinRM

```hcl
terraform {
  required_providers {
    dryad = {
      source = "bjoernf73/dryad"
    }
  }
}

provider "dryad" {
  transport        = "winrm"
  host             = "ad1.contoso.local"
  port             = 5986
  username         = "CONTOSO\\terraform"
  password         = var.winrm_password
  winrm_use_tls    = true
  winrm_auth       = "kerberos"
  winrm_kerberos_realm = "CONTOSO.LOCAL"
  powershell_path  = "pwsh"
}

resource "dryad_organizational_unit" "servers" {
  path        = "Contoso/Servers/Windows"
  description = "Windows server OU"
}
```

### SSH

```hcl
provider "dryad" {
  transport          = "ssh"
  host               = "ad1.contoso.local"
  port               = 22
  username           = "terraform"
  ssh_private_key_pem = var.ssh_private_key_pem
  powershell_path    = "pwsh"
}
```

## Notes

- PowerShell 7 is required on the target Windows host.
- For WinRM, the provider currently supports basic, NTLM, and Kerberos authentication.
- For SSH, the provider currently supports password or private key authentication.
