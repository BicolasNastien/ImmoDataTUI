// ImmoDataTUI — exploration des données DVF (Demandes de Valeurs Foncières)
//
// Les données sont récupérées à la demande depuis files.data.gouv.fr : on
// choisit une région sur la carte de France, puis un département, dont les
// millésimes sont téléchargés ; la carte devient alors celle du département,
// découpée en communes, et cliquer sur une commune ouvre ses statistiques.
//
// Les fichiers déposés à la main sous <racine>/<ville>/<année>/*.csv restent
// lisibles : la touche « l » ouvre cette collection locale.
//
// La touche « a » ouvre l'autre versant du marché : les biens actuellement en
// vente, interrogés en direct chez les portails d'annonces (voir annonces.go).
// DVF dit ce qui s'est vendu, les annonces disent ce qui est demandé — ce ne
// sont pas les mêmes chiffres, et le programme ne les mélange jamais.
//
// Usage :
//
//	immodata [racine]      (défaut : voir racineParDefaut)
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	racine, err := racineParDefaut()
	if len(os.Args) > 1 {
		racine, err = os.Args[1], nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dossier de données introuvable : %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(racine, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Impossible de créer %s : %v\n", racine, err)
		os.Exit(1)
	}

	// La collection locale n'est plus obligatoire : elle complète la carte, elle
	// ne la conditionne pas.
	villes, err := ScannerRacine(racine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Impossible de lire %s : %v\n", racine, err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		NouveauModele(racine, villes),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // survol et clic sur la carte
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur : %v\n", err)
		os.Exit(1)
	}
}

// racineParDefaut choisit où ranger les extraits quand aucun dossier n'est
// donné. Un dossier data/ déjà présent là où l'on lance le programme garde la
// priorité, pour qu'une collection existante continue de s'ouvrir. Sinon, on
// suit la convention de chaque plateforme : lancé d'un double-clic, le
// programme n'a pas de répertoire courant digne de ce nom.
func racineParDefaut() (string, error) {
	if info, err := os.Stat("data"); err == nil && info.IsDir() {
		return "data", nil
	}
	var base string
	switch runtime.GOOS {
	case "darwin":
		// ~/Library/Application Support
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	case "windows":
		// %LocalAppData% : des données volumineuses n'ont rien à faire dans
		// le profil itinérant.
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = dir
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "immodata", "data"), nil
}
