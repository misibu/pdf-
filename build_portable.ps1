param([switch]$SkipDownload)
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$work = Join-Path $root ".release-tmp"
$downloads = Join-Path $work "downloads"
$unpacked = Join-Path $work "extracted"
$staging = Join-Path $work "bundle"
$eciFolder = Join-Path $work "eci"
$dist = Join-Path $root "dist"
$payload = Join-Path $root "assets\ghostscript_bundle.zip"
$gsUrl = "https://github.com/ArtifexSoftware/ghostpdl-downloads/releases/download/gs10080/gs10080w64.exe"
$gsSHA256 = "52a91b8bf09298788d7a57b9206127026c23eacd75405f0a131e26dc381dce50"
$eciUrl = "https://www.eci.org/lib/exe/fetch.php?media=downloads:icc_profiles_from_eci:eci_offset_2009.zip"

function Fetch([string]$url, [string]$dest) {
    if ($SkipDownload -and (Test-Path -LiteralPath $dest)) { return }
    for ($i = 1; $i -le 3; $i++) {
        try {
            Invoke-WebRequest -Uri $url -OutFile $dest -MaximumRedirection 10
            return
        } catch {
            if ($i -eq 3) { throw }
            Start-Sleep -Seconds (2 * $i)
        }
    }
}

New-Item -ItemType Directory -Force -Path $work,$downloads,$dist,(Join-Path $root 'assets') | Out-Null
$installer = Join-Path $downloads "gs10080w64.exe"
$eciZip = Join-Path $downloads "eci_offset_2009.zip"

Write-Host "Downloading verified Ghostscript 10.08.0..."
Fetch $gsUrl $installer
$actualSHA = (Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualSHA -ne $gsSHA256) {
    throw "Ghostscript SHA256 mismatch: $actualSHA"
}

Write-Host "Downloading ECI ISO Coated v2 FOGRA39 ICC profile..."
Fetch $eciUrl $eciZip

$sevenZip = Get-Command 7z.exe -ErrorAction SilentlyContinue
if ($sevenZip) { $sevenZipPath = $sevenZip.Source }
else { $sevenZipPath = Join-Path $env:ProgramFiles '7-Zip\7z.exe' }
if (!(Test-Path -LiteralPath $sevenZipPath)) { throw "7-Zip is unavailable on build runner" }
foreach ($dir in @($unpacked,$staging,$eciFolder)) {
    if (Test-Path -LiteralPath $dir) { Remove-Item -LiteralPath $dir -Recurse -Force }
    New-Item -ItemType Directory -Path $dir | Out-Null
}
Write-Host "Unpacking official Windows executable..."
& $sevenZipPath x -y "-o$unpacked" $installer | Out-Null
if ($LASTEXITCODE -ne 0) { throw "7-Zip failed to unpack Ghostscript installer" }

$gsExe = Get-ChildItem -LiteralPath $unpacked -Recurse -File -Filter "gswin64c.exe" | Select-Object -First 1
if (!$gsExe) { throw "Official Ghostscript installer contains no gswin64c.exe" }
$gsDLL = Join-Path $gsExe.DirectoryName "gsdll64.dll"
if (!(Test-Path -LiteralPath $gsDLL)) { throw "Missing gsdll64.dll" }
New-Item -ItemType Directory -Path (Join-Path $staging 'bin') | Out-Null
Copy-Item $gsExe.FullName (Join-Path $staging 'bin\gswin64c.exe')
Get-ChildItem -LiteralPath $gsExe.DirectoryName -File -Filter '*.dll' | ForEach-Object {
    Copy-Item $_.FullName (Join-Path (Join-Path $staging 'bin') $_.Name)
}
$resource = Get-ChildItem -LiteralPath $unpacked -Recurse -Directory -Filter Resource |
    Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'Init') } | Select-Object -First 1
if ($resource) { Copy-Item $resource.FullName (Join-Path $staging 'Resource') -Recurse }
$lib = Get-ChildItem -LiteralPath $unpacked -Recurse -Directory -Filter lib |
    Where-Object { (Test-Path (Join-Path $_.FullName 'gs_init.ps')) -or (Test-Path (Join-Path $_.FullName 'pdf2dsc.ps')) } |
    Select-Object -First 1
if ($lib) { Copy-Item $lib.FullName (Join-Path $staging 'lib') -Recurse }
$fonts = Get-ChildItem -LiteralPath $unpacked -Recurse -Directory -Filter fonts | Select-Object -First 1
if ($fonts) { Copy-Item $fonts.FullName (Join-Path $staging 'fonts') -Recurse }

Expand-Archive -LiteralPath $eciZip -DestinationPath $eciFolder -Force
$icc = Get-ChildItem -LiteralPath $eciFolder -Recurse -File -Filter ISOcoated_v2_eci.icc | Select-Object -First 1
if (!$icc) { throw "FOGRA39 ISOcoated_v2_eci.icc missing from ECI download" }
$iccBytes = [IO.File]::ReadAllBytes($icc.FullName)
if ($iccBytes.Length -lt 128 -or [Text.Encoding]::ASCII.GetString($iccBytes,16,4) -ne 'CMYK' -or
    [Text.Encoding]::ASCII.GetString($iccBytes,36,4) -ne 'acsp') { throw "Invalid CMYK ICC profile" }
New-Item -ItemType Directory -Path (Join-Path $staging 'profiles') | Out-Null
Copy-Item $icc.FullName (Join-Path $staging 'profiles\ISOcoated_v2_eci.icc')

@'
Ghostscript 10.08.0 (c) Artifex Software: AGPLv3 or commercial license.
Source: https://github.com/ArtifexSoftware/ghostpdl/tree/gs10080
License: https://www.gnu.org/licenses/agpl-3.0.html
ISO Coated v2 (FOGRA39) ICC profile: ECI, https://www.eci.org/
Components extract into a per-user cache on first conversion; there is no installer.
'@ | Set-Content -LiteralPath (Join-Path $staging 'THIRD_PARTY_NOTICE.txt') -Encoding UTF8
Fetch "https://www.gnu.org/licenses/agpl-3.0.txt" (Join-Path $staging 'AGPL-3.0.txt')

if (Test-Path -LiteralPath $payload) { Remove-Item $payload -Force }
Compress-Archive -Path (Join-Path $staging '*') -DestinationPath $payload -CompressionLevel Optimal -Force
$payloadSize = (Get-Item $payload).Length
if ($payloadSize -lt 1MB) { throw "Embedded Ghostscript payload is too small: $payloadSize" }
Write-Host "Bundled payload size: $([math]::Round($payloadSize/1MB,1)) MiB"

Push-Location $root
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags="-H windowsgui -s -w" -o (Join-Path $dist 'PDF_Color_Checker_5.4_Portable.exe') .
    if ($LASTEXITCODE -ne 0) { throw "Go build failed: $LASTEXITCODE" }
} finally { Pop-Location }
Write-Host "SUCCESS: dist\PDF_Color_Checker_5.4_Portable.exe"
