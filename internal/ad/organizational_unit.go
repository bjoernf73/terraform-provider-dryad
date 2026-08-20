package ad

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/client"
)

type OrganizationalUnit struct {
	Path              string  `json:"path"`
	Description       *string `json:"description"`
	DistinguishedName string  `json:"distinguished_name"`
	Name              string  `json:"name"`
	Exists            bool    `json:"exists"`
}

func EnsureOrganizationalUnit(ctx context.Context, c *client.Client, path string, description *string) (*OrganizationalUnit, error) {
	script, err := buildScript(c, ensureOrganizationalUnitBody, map[string]any{
		"path":        NormalizePath(path),
		"description": description,
	})
	if err != nil {
		return nil, err
	}

	var result OrganizationalUnit
	if err := c.RunPowerShellJSON(ctx, script, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func ReadOrganizationalUnit(ctx context.Context, c *client.Client, distinguishedName string) (*OrganizationalUnit, error) {
	script, err := buildScript(c, readOrganizationalUnitBody, map[string]any{
		"distinguished_name": distinguishedName,
	})
	if err != nil {
		return nil, err
	}

	var result OrganizationalUnit
	if err := c.RunPowerShellJSON(ctx, script, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func UpdateOrganizationalUnitDescription(ctx context.Context, c *client.Client, distinguishedName string, description *string) (*OrganizationalUnit, error) {
	script, err := buildScript(c, updateOrganizationalUnitBody, map[string]any{
		"distinguished_name": distinguishedName,
		"description":        description,
	})
	if err != nil {
		return nil, err
	}

	var result OrganizationalUnit
	if err := c.RunPowerShellJSON(ctx, script, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func DeleteOrganizationalUnit(ctx context.Context, c *client.Client, distinguishedName string, deleteSubtree bool) error {
	script, err := buildScript(c, deleteOrganizationalUnitBody, map[string]any{
		"distinguished_name": distinguishedName,
		"delete_subtree":     deleteSubtree,
	})
	if err != nil {
		return err
	}

	var result map[string]any
	return c.RunPowerShellJSON(ctx, script, &result)
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")
	return path
}

func buildScript(c *client.Client, body string, payload map[string]any) (string, error) {
	payload["domain_controller"] = c.Config().DomainController

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encoding organizational unit payload: %w", err)
	}

	return organizationalUnitCommonScript +
		fmt.Sprintf("\n$payloadJson = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))\n$payload = $payloadJson | ConvertFrom-Json -ErrorAction Stop\n", base64.StdEncoding.EncodeToString(jsonPayload)) +
		body, nil
}

const organizationalUnitCommonScript = `
$ErrorActionPreference = 'Stop'
Import-Module ActiveDirectory -ErrorAction Stop

function Get-ServerParams {
    $serverParams = @{}
    if ($null -ne $payload.domain_controller -and -not [string]::IsNullOrWhiteSpace([string]$payload.domain_controller)) {
        $serverParams['Server'] = [string]$payload.domain_controller
    }
    return $serverParams
}

function Get-DomainDN {
    $serverParams = Get-ServerParams
    return (Get-ADDomain @serverParams -ErrorAction Stop).DistinguishedName
}

function Convert-PathToDN([string]$Path, [string]$DomainDN) {
    $segments = @($Path -split '/' | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
    if ($segments.Count -eq 0) {
        throw 'path must contain at least one OU segment'
    }

    [array]::Reverse($segments)
    $ouParts = $segments | ForEach-Object { 'OU=' + $_ }
    return ($ouParts -join ',') + ',' + $DomainDN
}

function Convert-DNToPath([string]$DistinguishedName, [string]$DomainDN) {
    $relativeDN = $DistinguishedName
    if ($relativeDN.EndsWith(',' + $DomainDN)) {
        $relativeDN = $relativeDN.Substring(0, $relativeDN.Length - ($DomainDN.Length + 1))
    }

    $segments = @($relativeDN -split ',' | ForEach-Object {
        if ($_.StartsWith('OU=')) {
            $_.Substring(3)
        }
        else {
            $_
        }
    })

    [array]::Reverse($segments)
    return ($segments -join '/')
}

function Test-IsIdentityNotFound([System.Management.Automation.ErrorRecord]$ErrorRecord) {
    if ($null -eq $ErrorRecord) {
        return $false
    }

    if ($ErrorRecord.CategoryInfo.Reason -eq 'ADIdentityNotFoundException') {
        return $true
    }

    if ($null -ne $ErrorRecord.Exception -and $ErrorRecord.Exception.Message -match 'Cannot find an object with identity') {
        return $true
    }

    return $false
}

function Get-LeafOrganizationalUnit([string]$DistinguishedName) {
    $serverParams = Get-ServerParams
    return Get-ADOrganizationalUnit -Identity $DistinguishedName -Properties Description, DistinguishedName, Name @serverParams -ErrorAction Stop
}
`

const ensureOrganizationalUnitBody = `
$domainDN = Get-DomainDN
$segments = @([string]$payload.path -split '/' | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
if ($segments.Count -eq 0) {
    throw 'path must contain at least one OU segment'
}

$serverParams = Get-ServerParams
$parentDN = $domainDN
foreach ($segment in $segments) {
    $currentDN = 'OU=' + $segment + ',' + $parentDN

    try {
        Get-ADObject -Identity $currentDN @serverParams -ErrorAction Stop | Out-Null
    }
    catch {
        if (-not (Test-IsIdentityNotFound $_)) {
            throw
        }

        New-ADOrganizationalUnit -Name $segment -Path $parentDN @serverParams -ErrorAction Stop | Out-Null
    }

    $parentDN = $currentDN
}

$leafDN = Convert-PathToDN ([string]$payload.path) $domainDN
if ($payload.PSObject.Properties.Name -contains 'description') {
    if ($null -eq $payload.description -or [string]::IsNullOrWhiteSpace([string]$payload.description)) {
        Set-ADOrganizationalUnit -Identity $leafDN -Clear Description @serverParams -ErrorAction Stop
    }
    else {
        Set-ADOrganizationalUnit -Identity $leafDN -Description ([string]$payload.description) @serverParams -ErrorAction Stop
    }
}

$ou = Get-LeafOrganizationalUnit $leafDN
[pscustomobject]@{
    exists = $true
    path = Convert-DNToPath $ou.DistinguishedName $domainDN
    description = $ou.Description
    distinguished_name = $ou.DistinguishedName
    name = $ou.Name
} | ConvertTo-Json -Compress
`

const readOrganizationalUnitBody = `
$domainDN = Get-DomainDN

try {
    $ou = Get-LeafOrganizationalUnit ([string]$payload.distinguished_name)
    [pscustomobject]@{
        exists = $true
        path = Convert-DNToPath $ou.DistinguishedName $domainDN
        description = $ou.Description
        distinguished_name = $ou.DistinguishedName
        name = $ou.Name
    } | ConvertTo-Json -Compress
}
catch {
    if (Test-IsIdentityNotFound $_) {
        [pscustomobject]@{
            exists = $false
        } | ConvertTo-Json -Compress
    }
    else {
        throw
    }
}
`

const updateOrganizationalUnitBody = `
$domainDN = Get-DomainDN
$serverParams = Get-ServerParams
$leafDN = [string]$payload.distinguished_name
$null = Get-LeafOrganizationalUnit $leafDN

if ($null -eq $payload.description -or [string]::IsNullOrWhiteSpace([string]$payload.description)) {
    Set-ADOrganizationalUnit -Identity $leafDN -Clear Description @serverParams -ErrorAction Stop
}
else {
    Set-ADOrganizationalUnit -Identity $leafDN -Description ([string]$payload.description) @serverParams -ErrorAction Stop
}

$ou = Get-LeafOrganizationalUnit $leafDN
[pscustomobject]@{
    exists = $true
    path = Convert-DNToPath $ou.DistinguishedName $domainDN
    description = $ou.Description
    distinguished_name = $ou.DistinguishedName
    name = $ou.Name
} | ConvertTo-Json -Compress
`

const deleteOrganizationalUnitBody = `
$serverParams = Get-ServerParams
$leafDN = [string]$payload.distinguished_name

try {
    $null = Get-LeafOrganizationalUnit $leafDN
}
catch {
    if (Test-IsIdentityNotFound $_) {
        [pscustomobject]@{
            deleted = $false
            exists = $false
        } | ConvertTo-Json -Compress
        return
    }

    throw
}

Set-ADOrganizationalUnit -Identity $leafDN -ProtectedFromAccidentalDeletion:$false @serverParams -ErrorAction Stop

if ($payload.delete_subtree -eq $true) {
    Remove-ADOrganizationalUnit -Identity $leafDN -Recursive -Confirm:$false @serverParams -ErrorAction Stop
}
else {
    Remove-ADOrganizationalUnit -Identity $leafDN -Confirm:$false @serverParams -ErrorAction Stop
}

[pscustomobject]@{
    deleted = $true
    exists = $false
} | ConvertTo-Json -Compress
`
