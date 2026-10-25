// dvf-tui — exploration des données DVF (Demandes de Valeurs Foncières)
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
//	dvf-tui [racine]      (défaut : data)
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const racineParDefaut = "data"

func main() {
	racine := racineParDefaut
	if len(os.Args) > 1 {
		racine = os.Args[1]
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

