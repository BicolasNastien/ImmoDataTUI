#!/bin/sh
# Installe dvf-tui depuis les releases GitHub, sous Linux et macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/BicolasNastien/ImmoDataTUI/main/install.sh | sh
#
# Variables facultatives :
#   VERSION=0.1.0           version à installer (défaut : la dernière)
#   INSTALL_DIR=/chemin     dossier de destination (défaut : ~/.local/bin)
set -eu

REPO="BicolasNastien/ImmoDataTUI"
BINAIRE="dvf-tui"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

erreur() {
	echo "install.sh : $*" >&2
	exit 1
}

# Tout le travail est dans une fonction appelée à la dernière ligne : si
# `curl | sh` est interrompu en cours de route, le shell ne reçoit qu'une
# définition incomplète et n'exécute rien.
main() {
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

main "$@"
