# Windows amd64 release: portable EXE, installer, SHA256SUMS.
# Requires Go and Inno Setup 6. Run from any directory.
param([string]$ISCC = "")
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$resource = Join-Path $root "moor_windows_amd64.syso"
$hadResource = Test-Path $resource
$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
try {
    $source = Get-Content main.go -Raw
    if ($source -notmatch 'const version = "(\d+\.\d+\.\d+)"') { throw "Version not found in main.go" }
    $version = $Matches[1]
    $parts = $version.Split('.')
    $out = Join-Path $root "dist\windows"
    $build = Join-Path $out "build"
    New-Item $build -ItemType Directory -Force | Out-Null
    if (Test-Path $resource) { throw "Existing resource file: $resource (remove it before building)" }

    if (!$ISCC) {
        $command = Get-Command ISCC.exe -ErrorAction SilentlyContinue
        if ($command) { $ISCC = $command.Source }
        else {
            foreach ($base in @("$env:LOCALAPPDATA\Programs", ${env:ProgramFiles(x86)}, $env:ProgramFiles)) {
                $candidate = Join-Path $base "Inno Setup 6\ISCC.exe"
                if (Test-Path $candidate) { $ISCC = $candidate; break }
            }
        }
    }
    if (!$ISCC -or !(Test-Path $ISCC)) { throw "Install Inno Setup 6: winget install --id JRSoftware.InnoSetup --exact" }

    # ICO containing the repository's PNG artwork (no external image tool).
    Add-Type -AssemblyName System.Drawing
    $pngPath = Join-Path $root 'dist\moor.png'
    $image = [Drawing.Image]::FromFile($pngPath)
    try { $width = $image.Width; $height = $image.Height } finally { $image.Dispose() }
    if ($width -gt 256 -or $height -gt 256) { throw 'Icon must be at most 256x256' }
    $png = [IO.File]::ReadAllBytes($pngPath)
    $icon = Join-Path $build 'moor.ico'
    $stream = [IO.File]::Create($icon)
    $writer = New-Object IO.BinaryWriter($stream)
    try {
        $writer.Write([uint16]0); $writer.Write([uint16]1); $writer.Write([uint16]1)
        $writer.Write([byte]($width % 256)); $writer.Write([byte]($height % 256))
        $writer.Write([byte]0); $writer.Write([byte]0)
        $writer.Write([uint16]1); $writer.Write([uint16]32)
        $writer.Write([uint32]$png.Length); $writer.Write([uint32]22)
        $writer.Write($png)
    } finally { $writer.Dispose() }

    $metadata = @{
        IconPath = $icon
        FixedFileInfo = @{
            FileVersion = @{ Major = [int]$parts[0]; Minor = [int]$parts[1]; Patch = [int]$parts[2]; Build = 0 }
            ProductVersion = @{ Major = [int]$parts[0]; Minor = [int]$parts[1]; Patch = [int]$parts[2]; Build = 0 }
            FileType = "01"
        }
        StringFileInfo = @{
            CompanyName = "DolbyDAX2"; FileDescription = "Moor SSH Server Manager"
            FileVersion = $version; ProductVersion = $version; ProductName = "Moor"
            OriginalFilename = "moor.exe"; InternalName = "moor"
        }
        VarFileInfo = @{ Translation = @{ LangID = "0409"; CharsetID = "04B0" } }
    }
    $json = Join-Path $build "versioninfo.json"
    [IO.File]::WriteAllText($json, ($metadata | ConvertTo-Json -Depth 6), (New-Object Text.UTF8Encoding($false)))
    # Pinned resource generator; does not change the project's go.mod.
    & go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0 -64 -o $resource $json
    if ($LASTEXITCODE) { throw "Version resource generation failed" }
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    & go build -trimpath -tags production -ldflags '-H windowsgui' -o "$build\moor.exe" .
    if ($LASTEXITCODE) { throw "Windows build failed" }
    Copy-Item README.md $build -Force
    $portable = "moor-$version-windows-amd64.exe"
    Copy-Item "$build\moor.exe" "$out\$portable" -Force

    # Only Microsoft's signed WebView2 bootstrapper is bundled.
    $bootstrap = Join-Path $build "MicrosoftEdgeWebview2Setup.exe"
    Invoke-WebRequest 'https://go.microsoft.com/fwlink/p/?LinkId=2124703' -OutFile $bootstrap -UseBasicParsing
    $signature = Get-AuthenticodeSignature $bootstrap
    if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') {
        throw "WebView2 bootstrapper signature verification failed"
    }
    & $ISCC "/DAppVersion=$version" "/DSourceDir=$build" "/DOutputPath=$out" "packaging\windows\moor.iss"
    if ($LASTEXITCODE) { throw "Installer compilation failed" }
    $installer = "moor-$version-windows-amd64-setup.exe"
    $checksums = foreach ($name in @($portable, $installer)) {
        $hash = (Get-FileHash (Join-Path $out $name) -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $name"
    }
    [IO.File]::WriteAllText((Join-Path $out 'SHA256SUMS.txt'), (($checksums -join "`n") + "`n"), (New-Object Text.UTF8Encoding($false)))
    Write-Host "Release artifacts: $out"
} finally {
    if (!$hadResource) { Remove-Item $resource -ErrorAction SilentlyContinue }
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
    Pop-Location
}
