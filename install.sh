#!/bin/sh
# Installe ou désinstalle ImmoDataTUI depuis les releases GitHub, sous Linux et macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh -s -- --uninstall
#
# Options :
#   --uninstall             supprimer le binaire, puis demander s'il faut
#                           supprimer aussi les données et le cache
#   --purge                 avec --uninstall : tout supprimer sans demander
#   --keep-data             avec --uninstall : garder données et cache sans demander
#
# Variables facultatives :
#   VERSION=0.2.0           version à installer (défaut : la dernière)
#   INSTALL_DIR=/chemin     dossier du binaire (défaut : ~/.local/bin)
set -eu

REPO="BicolasNastien/ImmoDataTUI"
BINAIRE="immodata"
ANCIEN_NOM="dvf-tui" # nom d'avant ImmoDataTUI : la désinstallation nettoie aussi ses restes
INSTALL_DIR_DONNE="${INSTALL_DIR:-}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

erreur() {
	echo "install.sh : $*" >&2
	exit 1
}

# Tout le travail est dans une fonction appelée à la dernière ligne : si
# `curl | sh` est interrompu en cours de route, le shell ne reçoit qu'une
# définition incomplète et n'exécute rien.
main() {
	mode=installer
	donnees=demander
	for arg in "$@"; do
		case "$arg" in
		--uninstall) mode=desinstaller ;;
		--purge) donnees=supprimer ;;
		--keep-data) donnees=garder ;;
		-h | --help)
			echo "Usage : install.sh [--uninstall [--purge | --keep-data]]"
			echo "Variables : VERSION=x.y.z, INSTALL_DIR=/chemin"
			return
			;;
		*) erreur "option inconnue : $arg" ;;
		esac
	done

	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) erreur "système non pris en charge : $(uname -s). Sous Windows, utiliser install.ps1." ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) erreur "processeur non pris en charge : $(uname -m)" ;;
	esac

	if [ "$mode" = desinstaller ]; then
		desinstaller
		return
	fi

	command -v curl >/dev/null 2>&1 || erreur "curl est nécessaire"
	command -v tar >/dev/null 2>&1 || erreur "tar est nécessaire"

	# La dernière version se lit dans la redirection de /releases/latest : pas
	# d'appel à l'API GitHub, donc pas de limite de requêtes à craindre.
	if [ -z "${VERSION:-}" ]; then
		url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
			erreur "impossible de joindre github.com"
		VERSION=${url##*/v}
		[ -n "$VERSION" ] && [ "$VERSION" != "$url" ] || erreur "aucune release publiée"
	fi
	VERSION=${VERSION#v}

	archive="${BINAIRE}_${VERSION}_${os}_${arch}.tar.gz"
	base="https://github.com/$REPO/releases/download/v$VERSION"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT

	echo "Téléchargement de $BINAIRE $VERSION ($os/$arch)…"
	curl -fsSL -o "$tmp/$archive" "$base/$archive" || erreur "archive introuvable : $base/$archive"
	curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || erreur "checksums.txt introuvable"

	attendu=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
	[ -n "$attendu" ] || erreur "$archive absent de checksums.txt"
	if command -v sha256sum >/dev/null 2>&1; then
		obtenu=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
	else
		obtenu=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
	fi
	[ "$attendu" = "$obtenu" ] || erreur "somme de contrôle invalide pour $archive"

	tar -xzf "$tmp/$archive" -C "$tmp" "$BINAIRE"
	mkdir -p "$INSTALL_DIR"
	install -m 755 "$tmp/$BINAIRE" "$INSTALL_DIR/$BINAIRE" 2>/dev/null ||
		erreur "impossible d'écrire dans $INSTALL_DIR (essayer INSTALL_DIR=… ou sudo)"

	# Téléchargé par curl, le binaire n'est pas en quarantaine ; on retire tout de
	# même l'attribut au cas où il aurait été hérité.
	if [ "$os" = darwin ]; then
		xattr -d com.apple.quarantine "$INSTALL_DIR/$BINAIRE" 2>/dev/null || true
	fi

	echo "$BINAIRE $VERSION installé dans $INSTALL_DIR/$BINAIRE"

	case ":$PATH:" in
	*":$INSTALL_DIR:"*) echo "Lancer : $BINAIRE" ;;
	*)
		echo
		echo "$INSTALL_DIR n'est pas dans le PATH. Ajouter à ~/.bashrc ou ~/.zshrc :"
		echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
	esac
}

# confirmer pose une question au terminal et réussit sur « oui ». Avec
# `curl | sh`, l'entrée standard est le script lui-même : il faut lire
# /dev/tty. Sans terminal (CI, cron), la réponse est « non ».
confirmer() {
	(: </dev/tty) 2>/dev/null || return 1
	printf '%s [o/N] ' "$1" >/dev/tty
	read -r reponse </dev/tty || return 1
	case "$reponse" in
	o | O | oui | Oui | y | Y | yes) return 0 ;;
	esac
	return 1
}

desinstaller() {
	# Le binaire : là où on l'a demandé, sinon à l'emplacement par défaut,
	# sinon là où le PATH le trouve.
	trouve=0
	for nom in "$BINAIRE" "$ANCIEN_NOM"; do
		if [ -n "$INSTALL_DIR_DONNE" ] || [ -e "$INSTALL_DIR/$nom" ]; then
			cible="$INSTALL_DIR/$nom"
		else
			cible=$(command -v "$nom" 2>/dev/null || true)
		fi
		if [ -n "$cible" ] && [ -e "$cible" ]; then
			rm -f "$cible" 2>/dev/null || erreur "impossible de supprimer $cible (essayer avec sudo)"
			echo "Binaire supprimé : $cible"
			trouve=1
		fi
	done
	if [ "$trouve" = 0 ]; then
		echo "Aucun binaire $BINAIRE trouvé (préciser INSTALL_DIR s'il a été installé ailleurs)."
	fi

	dossiers=""
	for nom in "$BINAIRE" "$ANCIEN_NOM"; do
		if [ "$os" = darwin ]; then
			dossiers="$dossiers$HOME/Library/Application Support/$nom
$HOME/Library/Caches/$nom
"
		else
			dossiers="$dossiers${XDG_DATA_HOME:-$HOME/.local/share}/$nom
${XDG_CACHE_HOME:-$HOME/.cache}/$nom
"
		fi
	done

	presents=""
	anciennes_ifs=$IFS
	IFS='
'
	for d in $dossiers; do
		[ -d "$d" ] && presents="$presents$d
"
	done
	if [ -z "$presents" ]; then
		IFS=$anciennes_ifs
		echo "Aucune donnée locale à supprimer."
		return
	fi

	echo
	echo "Données locales (extraits par commune, archives téléchargées, contours, annonces vues) :"
	for d in $presents; do
		echo "  $(du -sh "$d" 2>/dev/null | cut -f1)	$d"
	done

	case "$donnees" in
	supprimer) effacer=1 ;;
	garder) effacer=0 ;;
	*) if confirmer "Supprimer aussi ces données ?"; then effacer=1; else effacer=0; fi ;;
	esac

	if [ "$effacer" = 1 ]; then
		for d in $presents; do
			rm -rf "$d"
			echo "Supprimé : $d"
		done
	else
		echo "Données conservées. Pour les supprimer plus tard : relancer avec --uninstall --purge"
	fi
	IFS=$anciennes_ifs

	echo
	echo "Un dossier data/ créé là où vous lanciez $BINAIRE n'est pas concerné : le supprimer à la main s'il existe."
}

main "$@"
