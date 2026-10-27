# Installation

ImmoDataTUI est un programme unique, sans dépendance : on télécharge l'archive de
son système, on l'extrait, on lance `immodata` dans un terminal. Rien d'autre à
installer.

- [Installation rapide](#installation-rapide)
- [Choisir son archive](#choisir-son-archive)
- [Linux](#linux)
- [macOS](#macos)
- [Windows](#windows)
- [Depuis les sources](#depuis-les-sources)
- [Mettre à jour](#mettre-à-jour)
- [Désinstaller](#désinstaller)
- [Dépannage](#dépannage)

## Installation rapide

Un script détecte le système et le processeur, télécharge la dernière release,
vérifie sa somme de contrôle et installe le binaire.

**Linux et macOS** — installe dans `~/.local/bin` :

```bash
curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh
```

**Windows** — dans PowerShell, installe dans `%LocalAppData%\Programs\immodata`
et l'ajoute au `PATH` :

```powershell
irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1 | iex
```

Puis, dans un nouveau terminal : `immodata`.

Deux variables permettent d'ajuster l'installation :

| Variable | Effet | Exemple |
|---|---|---|
| `VERSION` | installer une version précise plutôt que la dernière | `VERSION=0.2.0` |
| `INSTALL_DIR` | changer le dossier de destination | `INSTALL_DIR=/usr/local/bin` |

```bash
curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | VERSION=0.2.0 sh
curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sudo INSTALL_DIR=/usr/local/bin sh
```

```powershell
$env:INSTALL_DIR = "C:\outils"; irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1 | iex
```

Exécuter un script téléchargé revient à faire confiance à son auteur : on peut
lire [`install.sh`](../install.sh) et [`install.ps1`](../install.ps1) avant de
les lancer, ou suivre l'installation manuelle ci-dessous.

## Choisir son archive

Les archives sont publiées sur la page
[Releases](https://github.com/BicolasNastien/ImmoDataTUI/releases/latest) :

| Système | Processeur | Archive |
|---|---|---|
| Linux | Intel / AMD 64 bits | `immodata_<version>_linux_amd64.tar.gz` |
| Linux | ARM 64 bits (Raspberry Pi 4/5, serveurs ARM) | `immodata_<version>_linux_arm64.tar.gz` |
| macOS | Apple Silicon (M1 et suivants) | `immodata_<version>_darwin_arm64.tar.gz` |
| macOS | Intel | `immodata_<version>_darwin_amd64.tar.gz` |
| Windows | Intel / AMD 64 bits | `immodata_<version>_windows_amd64.zip` |
| Windows | ARM (Surface Pro X, Snapdragon) | `immodata_<version>_windows_arm64.zip` |

En cas de doute sur le processeur : `uname -m` sous Linux et macOS (`x86_64` →
amd64, `aarch64` ou `arm64` → arm64) ; sous Windows, *Paramètres → Système →
Informations → Type de système*.

Le fichier `checksums.txt` de la release contient l'empreinte SHA-256 de chaque
archive. Les commandes de vérification sont données pour chaque système
ci-dessous.

### Prérequis communs

- **Un terminal moderne**, qui gère les couleurs 256, la souris et l'Unicode.
  La carte est dessinée avec des demi-blocs `▀` : un terminal ou une police qui
  ne les connaît pas affiche des carrés vides.
- **Une fenêtre d'au moins 100 × 30 caractères** environ ; la carte et les
  tableaux s'adaptent à la taille, mais deviennent illisibles en dessous.
- **Un accès à Internet** pour télécharger les données. La carte de France
  s'affiche sans réseau (ses contours sont embarqués), mais tout le reste en
  dépend. Un proxy est pris en compte via les variables `HTTPS_PROXY` /
  `HTTP_PROXY`.

## Linux

```bash
VERSION=0.2.0
ARCH=amd64          # ou arm64
cd /tmp
curl -LO "https://github.com/BicolasNastien/ImmoDataTUI/releases/download/v${VERSION}/immodata_${VERSION}_linux_${ARCH}.tar.gz"
curl -LO "https://github.com/BicolasNastien/ImmoDataTUI/releases/download/v${VERSION}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing

tar -xzf "immodata_${VERSION}_linux_${ARCH}.tar.gz"
mkdir -p ~/.local/bin
mv immodata ~/.local/bin/
```

Vérifier que `~/.local/bin` est dans le `PATH` (`echo $PATH`) ; sinon, ajouter
`export PATH="$HOME/.local/bin:$PATH"` au `~/.bashrc` ou `~/.zshrc`. Pour une
installation pour tous les utilisateurs : `sudo mv immodata /usr/local/bin/`.

Puis lancer :

```bash
immodata
```

**Terminaux recommandés** : tous les terminaux courants conviennent (GNOME
Terminal, Konsole, Alacritty, kitty, foot, Ghostty, WezTerm…). La console
texte brute (TTY, `Ctrl+Alt+F3`) n'affiche que 8 couleurs et pas de souris.

**Ouvrir les liens** : les touches qui ouvrent un bien ou une annonce dans le
navigateur passent par `xdg-open` (paquet `xdg-utils`, présent sur toute
installation de bureau). Sur un serveur ou via SSH, le lien complet est affiché
à la place, pour être copié.

## macOS

```bash
VERSION=0.2.0
ARCH=arm64          # Apple Silicon ; amd64 pour un Mac Intel
cd /tmp
curl -LO "https://github.com/BicolasNastien/ImmoDataTUI/releases/download/v${VERSION}/immodata_${VERSION}_darwin_${ARCH}.tar.gz"
curl -LO "https://github.com/BicolasNastien/ImmoDataTUI/releases/download/v${VERSION}/checksums.txt"
shasum -a 256 -c checksums.txt --ignore-missing

tar -xzf "immodata_${VERSION}_darwin_${ARCH}.tar.gz"
sudo mkdir -p /usr/local/bin
sudo mv immodata /usr/local/bin/
```

Puis lancer `immodata` dans Terminal.

**Gatekeeper.** Le binaire n'est pas signé par Apple. Téléchargé avec `curl`
comme ci-dessus, il se lance directement. Téléchargé avec un navigateur, il est
mis en quarantaine et macOS affiche « impossible de vérifier le développeur ».
Pour le débloquer :

```bash
xattr -d com.apple.quarantine /usr/local/bin/immodata
```

ou, à la première ouverture refusée : *Réglages Système → Confidentialité et
sécurité → Ouvrir quand même*.

**Terminaux** : Terminal.app convient. iTerm2, Ghostty, WezTerm, kitty ou
Alacritty aussi. Dans Terminal.app, si les clics sur la carte ne font rien,
vérifier que le menu *Présentation → Autoriser le signalement de la souris*
(« Allow Mouse Reporting », `⌘R`) est coché.

Les liens s'ouvrent dans le navigateur par défaut via la commande `open`.

## Windows

1. Télécharger `immodata_<version>_windows_amd64.zip` (ou `arm64`) depuis la
   page [Releases](https://github.com/BicolasNastien/ImmoDataTUI/releases/latest).
2. *Clic droit → Extraire tout…*, puis déplacer `immodata.exe` dans un dossier
   stable, par exemple `%LocalAppData%\Programs\immodata\`.
3. Ajouter ce dossier au `PATH` pour pouvoir lancer `immodata` de n'importe où.
   Dans PowerShell :

   ```powershell
   $dir = "$env:LOCALAPPDATA\Programs\immodata"
   New-Item -ItemType Directory -Force $dir | Out-Null
   Move-Item .\immodata.exe $dir
   [Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
   ```

   Fermer et rouvrir le terminal pour que le `PATH` soit pris en compte.
4. Ouvrir **Windows Terminal** et taper `immodata`.

Vérifier l'archive (facultatif), dans PowerShell, puis comparer à la ligne
correspondante de `checksums.txt` :

```powershell
Get-FileHash .\immodata_0.2.0_windows_amd64.zip -Algorithm SHA256
```

**Utiliser Windows Terminal.** Il est installé d'office sur Windows 11 ; sous
Windows 10, il s'installe depuis le Microsoft Store. L'ancienne console
(`cmd.exe` ou PowerShell lancés seuls, fenêtre noire ou bleue sans onglets)
rend mal les couleurs, la souris et les caractères de la carte.

**Éviter le double-clic.** Lancé depuis l'explorateur, le programme s'ouvre dans
l'ancienne console. Mieux vaut le lancer depuis Windows Terminal.

**SmartScreen.** Le binaire n'est pas signé : à la première exécution, Windows
peut afficher « Windows a protégé votre ordinateur ». Cliquer sur *Informations
complémentaires → Exécuter quand même*.

## Depuis les sources

Il faut [Go](https://go.dev/dl/) 1.22 ou plus récent — de préférence la
dernière version stable : les binaires produits par Go 1.22.0 sont refusés par
les versions récentes de macOS.

```bash
git clone https://github.com/BicolasNastien/ImmoDataTUI.git
cd ImmoDataTUI/app
go build -o immodata .       # immodata.exe sous Windows
./immodata
```

La compilation croisée ne demande rien de plus, par exemple pour un Mac Apple
Silicon depuis Linux :

```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o immodata .
```

Les tests : `go test ./...` (ajouter `-short` pour sauter ceux qui sollicitent le
réseau).

## Mettre à jour

Relancer le script d'installation, ou refaire l'installation manuelle avec la
nouvelle version : le binaire remplace l'ancien.
Les données téléchargées et le cache sont conservés.

## Désinstaller

Le script d'installation sait aussi désinstaller. Il supprime le binaire (et,
sous Windows, son entrée du `PATH`), affiche la taille des données locales —
extraits par commune, archives téléchargées, contours, annonces déjà vues —
puis **demande s'il
faut les supprimer**. La réponse par défaut est non.

**Linux et macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh -s -- --uninstall
```

**Windows** (PowerShell)

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.ps1))) -Uninstall
```

| Option (sh / PowerShell) | Effet |
|---|---|
| `--purge` / `-Purge` | supprimer aussi les données et le cache, sans demander |
| `--keep-data` / `-KeepData` | garder les données et le cache, sans demander |

Sans terminal pour répondre (script, tâche planifiée), la question n'est pas
posée et les données sont conservées. La désinstallation retire aussi les restes de
l'ancien nom du programme, `dvf-tui` (binaire et dossiers), s'il en reste. Si le binaire a été installé ailleurs
qu'à l'emplacement par défaut, définir `INSTALL_DIR` comme à l'installation ;
à défaut, le script le cherche dans le `PATH`.

Pour désinstaller à la main : supprimer le binaire, puis, si on veut récupérer
la place, les données et le cache :

| Système | Données | Cache |
|---|---|---|
| Linux | `~/.local/share/immodata` (ou `$XDG_DATA_HOME/immodata`) | `~/.cache/immodata` (ou `$XDG_CACHE_HOME/immodata`) |
| macOS | `~/Library/Application Support/immodata` | `~/Library/Caches/immodata` |
| Windows | `%LocalAppData%\immodata` (données et cache sont dans le même dossier) | |

Si le programme a été lancé depuis un dossier contenant un sous-dossier `data/`,
les données sont dans ce `data/` (voir [Emplacements](sources.md#emplacements)).

## Dépannage

| Symptôme | Cause probable | Solution |
|---|---|---|
| La carte est faite de carrés ou de `?` | police sans les demi-blocs Unicode | choisir une police complète (Cascadia, JetBrains Mono, Fira Code, DejaVu Sans Mono, Menlo…) |
| Couleurs ternes ou fausses | terminal limité à 8 ou 16 couleurs | utiliser un terminal moderne ; sous Linux, vérifier que `TERM` vaut par exemple `xterm-256color` |
| Les clics et le survol ne font rien | le terminal ne transmet pas la souris | activer le signalement de la souris dans les réglages du terminal ; dans `tmux`, `set -g mouse on` |
| « Impossible d'ouvrir le navigateur » | pas d'environnement graphique (SSH, serveur) | copier le lien affiché |
| Téléchargements en échec | pas de réseau, proxy, pare-feu | vérifier l'accès à `files.data.gouv.fr` ; définir `HTTPS_PROXY` si besoin |
| Un département n'a aucune donnée | Moselle, Bas-Rhin, Haut-Rhin, Mayotte | ces départements ne sont pas couverts par DVF (voir [Sources](sources.md#couverture)) |
| macOS : « impossible de vérifier le développeur » | quarantaine Gatekeeper | `xattr -d com.apple.quarantine immodata` |
| macOS : `dyld … missing LC_UUID` | binaire compilé avec Go 1.22.0 | utiliser une release récente, ou recompiler avec la dernière version de Go |
| Windows : « Windows a protégé votre ordinateur » | SmartScreen, binaire non signé | *Informations complémentaires → Exécuter quand même* |
