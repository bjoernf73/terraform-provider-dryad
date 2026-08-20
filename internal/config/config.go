package config

import "time"

type Config struct {
	Host                string
	Port                int
	Transport           string
	Username            string
	Password            string
	Insecure            bool
	PowerShellPath      string
	DomainController    string
	Timeout             time.Duration
	WinRMUseTLS         bool
	WinRMAuth           string
	WinRMKerberosRealm  string
	WinRMKerberosConfig string
	WinRMKerberosSPN    string
	WinRMKerberosCCache string
	SSHPrivateKeyPEM    string
	SSHKnownHostsPath   string
	SSHHostKey          string
}
