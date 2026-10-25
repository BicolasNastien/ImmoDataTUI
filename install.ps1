# Installe ou désinstalle ImmoDataTUI depuis les releases GitHub, sous Windows.
#
#   irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1 | iex
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1))) -Uninstall
#
# Options :
#   -Uninstall   supprimer le binaire et son entrée du PATH, puis demander s'il
#                faut supprimer aussi les données et le cache
#   -Purge       avec -Uninstall : tout supprimer sans demander
#   -KeepData    avec -Uninstall : garder données et cache sans demander
#
# Variables facultatives (à définir avant la commande) :
#   $env:VERSION = "0.2.0"          version à installer (défaut : la dernière)
#   $env:INSTALL_DIR = "C:\outils"  dossier du binaire (défaut : %LocalAppData%\Programs\immodata)
param(
    [switch]$Uninstall,
    [switch]$Purge,
    [switch]$KeepData
)
$ErrorActionPreference = "Stop"

$Repo = "BicolasNastien/ImmoDataTUI"
$Binaire = "immodata"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\immodata" }
$AncienNom = "dvf-tui"   # nom d'avant ImmoDataTUI : la désinstallation nettoie aussi ses restes

function Installer {
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
}

function Desinstaller {
    $trouve = $false
    $dossiersDonnees = @()
    foreach ($nom in @($Binaire, $AncienNom)) {
        # Le binaire : là où on l'a demandé, sinon à l'emplacement par défaut,
        # sinon là où le PATH le trouve.
        $dir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\$nom" }
        $cible = Join-Path $dir "$nom.exe"
        if (-not $env:INSTALL_DIR -and -not (Test-Path $cible)) {
            $cmd = Get-Command "$nom.exe" -ErrorAction SilentlyContinue
            if ($cmd) { $cible = $cmd.Source }
        }
        if (Test-Path $cible) {
            Remove-Item $cible -Force
            Write-Host "Binaire supprimé : $cible"
            $trouve = $true
            $dossier = Split-Path $cible
            if (-not (Get-ChildItem $dossier -Force)) { Remove-Item $dossier -Force }

            $path = [Environment]::GetEnvironmentVariable("Path", "User")
            if (($path -split ";") -contains $dossier) {
                $entrees = $path -split ";" | Where-Object { $_ -and $_ -ne $dossier }
                [Environment]::SetEnvironmentVariable("Path", ($entrees -join ";"), "User")
                Write-Host "$dossier retiré du PATH"
            }
        }

        # Données et cache : os.UserCacheDir vaut %LocalAppData%.
        $d = Join-Path $env:LOCALAPPDATA $nom
        if (Test-Path $d) { $dossiersDonnees += $d }
    }
    if (-not $trouve) {
        Write-Host "Aucun binaire $Binaire trouvé (préciser `$env:INSTALL_DIR s'il a été installé ailleurs)."
    }

    if (-not $dossiersDonnees) {
        Write-Host "Aucune donnée locale à supprimer."
        return
    }

    Write-Host ""
    Write-Host "Données locales (extraits par commune, archives téléchargées, contours) :"
    foreach ($d in $dossiersDonnees) {
        $taille = (Get-ChildItem $d -Recurse -File -Force | Measure-Object Length -Sum).Sum
        Write-Host ("  {0,8:N1} Mo  {1}" -f ($taille / 1MB), $d)
    }

    # Sans console interactive (CI, tâche planifiée), on ne pose pas de
    # question : Read-Host attendrait indéfiniment.
    $effacer = $false
    if ($Purge) {
        $effacer = $true
    } elseif (-not $KeepData -and [Environment]::UserInteractive -and -not [Console]::IsInputRedirected) {
        $reponse = Read-Host "Supprimer aussi ces données ? [o/N]"
        $effacer = $reponse -match "^(o|oui|y|yes)$"
    }

    if ($effacer) {
        foreach ($d in $dossiersDonnees) {
            Remove-Item $d -Recurse -Force
            Write-Host "Supprimé : $d"
        }
    } else {
        Write-Host "Données conservées. Pour les supprimer plus tard : relancer avec -Uninstall -Purge"
    }

    Write-Host ""
    Write-Host "Un dossier data\ créé là où vous lanciez $Binaire n'est pas concerné : le supprimer à la main s'il existe."
}

if ($Uninstall) { Desinstaller } else { Installer }
