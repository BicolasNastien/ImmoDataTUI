# Installe dvf-tui depuis les releases GitHub, sous Windows.
#
#   irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1 | iex
#
# Variables facultatives (à définir avant la commande) :
#   $env:VERSION = "0.1.0"          version à installer (défaut : la dernière)
#   $env:INSTALL_DIR = "C:\outils"  dossier de destination (défaut : %LocalAppData%\Programs\dvf-tui)
$ErrorActionPreference = "Stop"

$Repo = "BicolasNastien/ImmoDataTUI"
$Binaire = "dvf-tui"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\dvf-tui" }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { throw "Processeur non pris en charge : $env:PROCESSOR_ARCHITECTURE" }
}

# La dernière version se lit dans la redirection de /releases/latest : pas
# d'appel à l'API GitHub, donc pas de limite de requêtes à craindre.
$version = $env:VERSION
if (-not $version) {
    $req = [System.Net.WebRequest]::Create("https://github.com/$Repo/releases/latest")
    $req.AllowAutoRedirect = $false
    $resp = $req.GetResponse()
    $version = ($resp.Headers["Location"] -split "/v")[-1]
    $resp.Close()
    if (-not $version) { throw "Aucune release publiée" }
}
$version = $version.TrimStart("v")

$archive = "${Binaire}_${version}_windows_${arch}.zip"
$base = "https://github.com/$Repo/releases/download/v$version"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null

try {
    Write-Host "Téléchargement de $Binaire $version (windows/$arch)…"
    $ProgressPreference = "SilentlyContinue"   # la barre de progression ralentit énormément Invoke-WebRequest
    Invoke-WebRequest "$base/$archive" -OutFile (Join-Path $tmp $archive) -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt") -UseBasicParsing

    $ligne = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { ($_ -split "\s+")[1] -eq $archive }
    if (-not $ligne) { throw "$archive absent de checksums.txt" }
    $attendu = ($ligne -split "\s+")[0]
    $obtenu = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLower()
    if ($attendu -ne $obtenu) { throw "Somme de contrôle invalide pour $archive" }

    Expand-Archive (Join-Path $tmp $archive) -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Force $InstallDir | Out-Null
    Copy-Item (Join-Path $tmp "$Binaire.exe") $InstallDir -Force
}
finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "$Binaire $version installé dans $InstallDir\$Binaire.exe"

$path = [Environment]::GetEnvironmentVariable("Path", "User")
if (($path -split ";") -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", "$path;$InstallDir", "User")
    Write-Host "$InstallDir ajouté au PATH : ouvrir un nouveau terminal, puis lancer $Binaire"
} else {
    Write-Host "Lancer : $Binaire"
}
