param(
    [string]$BridgeDllPath,
    [string]$OutputDir,
    [guid]$BasePolicyId = [guid]"60fd87f8-4593-44a0-91b0-2e0da022f248",
    [guid]$PolicyId = [guid]"6f31f6b4-ec77-4d7d-bcd0-4668f3790c71",
    [string]$PolicyName = "Spire2Mind Dev Supplemental"
)

$ErrorActionPreference = "Stop"

function Resolve-BridgeDir {
    Split-Path $PSScriptRoot -Parent
}

function Resolve-BridgeDllPath {
    param([string]$Candidate)

    if ($Candidate) {
        if (-not (Test-Path $Candidate)) {
            throw "Bridge DLL not found: $Candidate"
        }

        return (Resolve-Path $Candidate).Path
    }

    $bridgeDir = Resolve-BridgeDir
    $installed = "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2\mods\Spire2Mind.Bridge.dll"
    $built = Join-Path $bridgeDir "bin\Release\net9.0\Spire2Mind.Bridge.dll"

    foreach ($path in @($installed, $built)) {
        if (Test-Path $path) {
            return (Resolve-Path $path).Path
        }
    }

    throw "Could not resolve a Bridge DLL. Pass -BridgeDllPath explicitly."
}

function Resolve-OutputDir {
    param([string]$Candidate)

    if ($Candidate) {
        return $Candidate
    }

    return (Join-Path (Resolve-BridgeDir) "build\wdac")
}

function Require-Command {
    param([string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' is not available. Install the ConfigCI module / App Control tooling first."
    }
}

function Get-GuidLiteral {
    param([guid]$Guid)

    return "{0}" -f ("{" + $Guid.ToString().ToUpperInvariant() + "}")
}

function Update-PolicyXml {
    param(
        [string]$XmlPath,
        [guid]$ResolvedBasePolicyId,
        [guid]$ResolvedPolicyId
    )

    [xml]$document = Get-Content -LiteralPath $XmlPath
    $namespace = New-Object System.Xml.XmlNamespaceManager($document.NameTable)
    $namespace.AddNamespace("si", "urn:schemas-microsoft-com:sipolicy")

    $basePolicyNode = $document.SelectSingleNode("/si:SiPolicy/si:BasePolicyID", $namespace)
    $policyNode = $document.SelectSingleNode("/si:SiPolicy/si:PolicyID", $namespace)
    if (-not $basePolicyNode -or -not $policyNode) {
        throw "Generated policy XML is missing BasePolicyID or PolicyID."
    }

    $basePolicyNode.InnerText = Get-GuidLiteral -Guid $ResolvedBasePolicyId
    $policyNode.InnerText = Get-GuidLiteral -Guid $ResolvedPolicyId

    $idSettingNode = $document.SelectSingleNode("/si:SiPolicy/si:Settings/si:Setting[@Provider='PolicyInfo' and @Key='Information' and @ValueName='Id']/si:Value/si:String", $namespace)
    if ($idSettingNode) {
        $idSettingNode.InnerText = Get-GuidLiteral -Guid $ResolvedPolicyId
    }

    $document.Save($XmlPath)
}

Require-Command -Name "New-CIPolicyRule"
Require-Command -Name "New-CIPolicy"
Require-Command -Name "Set-CIPolicyIdInfo"
Require-Command -Name "ConvertFrom-CIPolicy"

$resolvedBridgeDllPath = Resolve-BridgeDllPath -Candidate $BridgeDllPath
$resolvedOutputDir = Resolve-OutputDir -Candidate $OutputDir
$policyIdLiteral = Get-GuidLiteral -Guid $PolicyId

New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

$xmlPath = Join-Path $resolvedOutputDir "Spire2Mind.Dev.Supplemental.xml"
$binaryPath = Join-Path $resolvedOutputDir ($policyIdLiteral + ".cip")

$rules = New-CIPolicyRule -DriverFilePath $resolvedBridgeDllPath -Level Publisher -Fallback Hash
New-CIPolicy -FilePath $xmlPath -Rules $rules -UserPEs -NoScript -MultiplePolicyFormat | Out-Null
Set-CIPolicyIdInfo -FilePath $xmlPath -PolicyName $PolicyName -SupplementsBasePolicyID $BasePolicyId | Out-Null
Update-PolicyXml -XmlPath $xmlPath -ResolvedBasePolicyId $BasePolicyId -ResolvedPolicyId $PolicyId
ConvertFrom-CIPolicy -XmlFilePath $xmlPath -BinaryFilePath $binaryPath | Out-Null

[pscustomobject]@{
    bridge_dll = $resolvedBridgeDllPath
    base_policy_id = $BasePolicyId.ToString()
    policy_id = $PolicyId.ToString()
    policy_name = $PolicyName
    xml_path = $xmlPath
    binary_path = $binaryPath
} | ConvertTo-Json -Compress
