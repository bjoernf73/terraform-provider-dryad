variable "host" {
  type = string
}

variable "port" {
  type    = number
  default = 5986
}

variable "username" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "winrm_use_tls" {
  type    = bool
  default = true
}

variable "winrm_auth" {
  type    = string
  default = "kerberos"
}

variable "winrm_kerberos_realm" {
  type = string
}
