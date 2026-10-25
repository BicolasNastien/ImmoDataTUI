package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func envoyer(m tea.Model, msgs ...tea.Msg) tea.Model {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func touche(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func sansCouleur(s string) string {
	var b strings.Builder
	dansCode := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			dansCode = true
		case dansCode && r == 'm':
			dansCode = false
		case !dansCode:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestNavigationCarte parcourt les trois échelons sans réseau : les messages de
// chargement sont injectés à la main, exactement comme les commandes les
// produiraient.
func TestNavigationCarte(t *testing.T) {
	regions, err := Regions()
	if err != nil {
		t.Fatal(err)
	}

	var m tea.Model = NouveauModele(t.TempDir(), nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 110, Height: 40},
		msgZones{niveau: nivRegions, zones: regions},
	)

	vue := sansCouleur(m.View())
	if !strings.Contains(vue, "quelle région") {
		t.Fatalf("écran carte des régions attendu, obtenu :\n%s", vue)
	}
	fmt.Println("### CARTE DES RÉGIONS ###")
	fmt.Println(m.View())

	// Nouvelle-Aquitaine est la 14e de la liste (triée par code).
	cible := -1
	for i, z := range regions {
		if z.Code == "75" {
			cible = i
		}
	}
	for i := 0; i < cible; i++ {
		m = envoyer(m, touche("down"))
	}
	if got := m.(model).carte.sel; got != cible {
		t.Fatalf("sélection %d attendue, obtenue %d", cible, got)
	}

	// Ouvrir la région : le modèle passe en attente et demande les départements.
	m = envoyer(m, touche("enter"))
	if m.(model).ecran != ecranAttente {
		t.Error("l'ouverture d'une région devrait afficher un écran d'attente")
	}

	deps, err := DepartementsDe("75")
	if err != nil {
		t.Fatal(err)
	}
	m = envoyer(m, msgZones{niveau: nivDepartements, zones: deps})
	vue = sansCouleur(m.View())
	if !strings.Contains(vue, "quel département") {
		t.Fatalf("écran carte des départements attendu, obtenu :\n%s", vue)
	}
	fmt.Println("### CARTE DES DÉPARTEMENTS ###")
	fmt.Println(m.View())

	// Un clic sur la carte sélectionne la zone pointée et l'ouvre.
	carte := m.(model).carte.carte
	col, ligne := -1, -1
	for l := 0; l < carte.H && ligne < 0; l++ {
		for c := 0; c < carte.L; c++ {
			if carte.ZoneEn(c, l) >= 0 {
				col, ligne = c, l
				break
			}
		}
	}
	if col < 0 {
		t.Fatal("aucune cellule cliquable sur la carte")
	}
	attendu := carte.ZoneEn(col, ligne)

	m = envoyer(m, tea.MouseMsg{
		X: col + margeGauche, Y: ligne + entetes,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	mm := m.(model)
	if mm.dep.Code != deps[attendu].Code {
		t.Errorf("clic : département %s attendu, obtenu %s", deps[attendu].Code, mm.dep.Code)
	}
	if mm.ecran != ecranTelechargement {
		t.Errorf("le clic sur un département devrait lancer le téléchargement")
	}

	// Résultats des millésimes, tels que les commandes les renverraient.
	for i, a := range mm.tele.annees {
		r := ResultatTelechargement{Dep: mm.dep.Code, Annee: a, Octets: 1 << 20}
		if i > 2 {
			r.Absent = true
		}
		m = envoyer(m, msgTelechargement(r))
	}
	fmt.Println("### TÉLÉCHARGEMENT ###")
	fmt.Println(m.View())
	if m.(model).ecran != ecranAttente {
		t.Error("après téléchargement, les contours des communes devraient être demandés")
	}
}

// TestParcoursStats vérifie l'enchaînement commune -> millésimes -> onglets.
func TestParcoursStats(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	archiveFactice(t, "2024", "64",
		"2024-1,2024-03-12,Vente,250000,RUE DES LILAS,64140,Boucau,Maison,90,4,641400000AB1,S,600",
		"2024-2,2024-05-02,Vente,310000,RUE DES LILAS,64140,Boucau,Maison,120,5,641400000AB2,S,800",
	)
	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}

	var m tea.Model = NouveauModele(racine, nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 110, Height: 40},
		msgExtraction{commune: Zone{Code: "64140", Nom: "Boucau"}, annees: annees},
	)
	if m.(model).ecran != ecranAnnees {
		t.Fatal("écran des millésimes attendu après extraction")
	}
	fmt.Println("### MILLÉSIMES ###")
	fmt.Println(m.View())

	m = envoyer(m, touche("enter"))
	if m.(model).ecran != ecranStats {
		t.Fatal("écran de statistiques attendu")
	}
	for i := range m.(model).onglets {
		fmt.Printf("### ONGLET %s ###\n", m.(model).onglets[i])
		fmt.Println(m.View())
		m = envoyer(m, touche("tab"))
	}

	if !strings.Contains(sansCouleur(m.View()), "Boucau") {
		t.Error("le titre devrait porter le nom de la commune")
	}

	// Retour : stats -> millésimes -> carte.
	m = envoyer(m, touche("esc"))
	if m.(model).ecran != ecranAnnees {
		t.Error("esc depuis les stats devrait revenir aux millésimes")
	}
	m = envoyer(m, touche("esc"))
	if m.(model).ecran != ecranCarte {
		t.Error("esc depuis les millésimes devrait revenir à la carte")
	}
}

// TestCollectionLocale vérifie que les fichiers déposés à la main restent
// accessibles depuis la carte.
func TestCollectionLocale(t *testing.T) {
	villes, err := ScannerRacine(racineDonnees)
	if err != nil || len(villes) == 0 {
		t.Skipf("aucune collection locale dans %s", racineDonnees)
	}
	regions, err := Regions()
	if err != nil {
		t.Fatal(err)
	}

	var m tea.Model = NouveauModele(racineDonnees, villes)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 110, Height: 40},
		msgZones{niveau: nivRegions, zones: regions},
		touche("l"),
	)
	if m.(model).ecran != ecranVilles {
		t.Fatal("la touche l devrait ouvrir la collection locale")
	}
	m = envoyer(m, touche("enter"), touche("enter"))
	if m.(model).ecran != ecranStats {
		t.Fatal("écran de statistiques attendu")
	}
	if m.(model).data.Ville != villes[0].Nom {
		t.Errorf("ville %s attendue, obtenue %s", villes[0].Nom, m.(model).data.Ville)
	}
}

// TestTableauEtFiche vérifie le tableau des ventes de l'onglet Maisons, le
// déplacement du curseur, le changement de tri et l'ouverture d'une fiche.
func TestTableauEtFiche(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	archiveFactice(t, "2024", "64",
		"2024-1,2024-03-12,Vente,250000,RUE DES LILAS,64140,Boucau,Maison,90,4,641400000AB1,S,600",
		"2024-2,2024-05-02,Vente,310000,AV DU PORT,64140,Boucau,Maison,120,5,641400000AB2,S,800",
		"2024-3,2024-01-20,Vente,190000,RUE NEUVE,64140,Boucau,Maison,70,3,641400000AB3,S,300",
	)
	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}

	var m tea.Model = NouveauModele(racine, nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: Zone{Code: "64140", Nom: "Boucau"}, annees: annees},
		touche("enter"), // millésime -> stats
		touche("tab"),   // Résumé -> Maisons
	)

	mm := m.(model)
	ventes := mm.ventesOnglet()
	if len(ventes) != 3 {
		t.Fatalf("3 maisons attendues dans le tableau, obtenues %d", len(ventes))
	}
	// Tri par défaut : la plus récente en tête.
	if ventes[0].ID != "2024-2" {
		t.Errorf("tri par date : 2024-2 attendue en tête, obtenue %s", ventes[0].ID)
	}
	fmt.Println("### ONGLET MAISONS AVEC TABLEAU ###")
	fmt.Println(m.View())

	// Tri par prix : le plus cher en tête, curseur remis au début.
	m = envoyer(m, touche("s"))
	if mm = m.(model); mm.tri != TriPrix || mm.curVente != 0 {
		t.Errorf("s devrait passer au tri par prix, obtenu %v", mm.tri)
	}

	// Descendre le curseur puis ouvrir la fiche.
	m = envoyer(m, touche("down"), touche("enter"))
	mm = m.(model)
	if mm.ecran != ecranVente {
		t.Fatal("↵ sur une ligne du tableau devrait ouvrir la fiche")
	}
	v, ok := mm.venteCourante()
	if !ok || v.ID != "2024-1" {
		t.Fatalf("fiche de 2024-1 attendue, obtenue %+v", v)
	}
	fmt.Println("### FICHE D'UNE VENTE ###")
	fmt.Println(m.View())

	vue := sansCouleur(m.View())
	for _, attendu := range []string{"RUE DES LILAS", "250 000 €", "641400000AB1", "Boucau", "MUTATION"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("la fiche devrait contenir %q", attendu)
		}
	}

	// n/p circulent d'une vente à l'autre sans repasser par le tableau.
	m = envoyer(m, touche("n"))
	if v, _ := m.(model).venteCourante(); v.ID != "2024-3" {
		t.Errorf("n devrait afficher la vente suivante, obtenue %s", v.ID)
	}
	m = envoyer(m, touche("p"), touche("p"))
	if v, _ := m.(model).venteCourante(); v.ID != "2024-2" {
		t.Errorf("p devrait remonter dans le tableau, obtenue %s", v.ID)
	}

	// esc revient au tableau, pas aux millésimes.
	m = envoyer(m, touche("esc"))
	if m.(model).ecran != ecranStats {
		t.Error("esc depuis la fiche devrait revenir au tableau")
	}

	// Clic sur une ligne : sélection, puis ouverture au second clic.
	mm = m.(model)
	y := mm.ligneTableau - mm.vp.YOffset + entetesStats + 2 // troisième ligne du tableau
	clic := tea.MouseMsg{X: 10, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m = envoyer(m, clic)
	if got := m.(model).curVente; got != 2 {
		t.Errorf("clic : vente 2 attendue, obtenue %d", got)
	}
	m = envoyer(m, clic)
	if m.(model).ecran != ecranVente {
		t.Error("un second clic sur la ligne sélectionnée devrait ouvrir la fiche")
	}
}

// TestChoixPeriode vérifie que la période se choisit dans la liste des
// millésimes : l'entrée « toutes les années » fusionne, les autres non.
func TestChoixPeriode(t *testing.T) {
	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2023", "64140",
		"2023-1,2023-04-11,Vente,240000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,",
		"2023-2,2023-06-02,Vente,150000,30,AV DU PORT,64140,64140,Boucau,Appartement,60,3,641400000AB7,,0,,",
	)
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,",
		"2024-2,2024-05-02,Vente,310000,4,RUE DES LILAS,64140,64140,Boucau,Maison,120,5,641400000AB2,S,800,,",
	)

	var m tea.Model = NouveauModele(racine, nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: Zone{Code: "64140", Nom: "Boucau"}, annees: []string{"2024", "2023"}},
	)
	entrees := m.(model).listeAnnees()
	if len(entrees) != 3 || !strings.Contains(entrees[0], "Toutes les années") {
		t.Fatalf("l'agrégat devrait ouvrir la liste, obtenu %v", entrees)
	}
	fmt.Println("### MILLÉSIMES ###")
	fmt.Println(m.View())

	// Première entrée : les deux millésimes fusionnés.
	m = envoyer(m, touche("enter"))
	got := m.(model)
	if !got.agrege || got.data.Annee != "2023 – 2024" || len(got.data.Sales) != 4 {
		t.Fatalf("agrégat attendu (2023 – 2024, 4 ventes), obtenu %v %q %d",
			got.agrege, got.data.Annee, len(got.data.Sales))
	}
	vue := sansCouleur(m.View())
	for _, attendu := range []string{"MILLÉSIMES FUSIONNÉS", "ÉVOLUTION — MAISONS"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("le résumé fusionné devrait contenir %q", attendu)
		}
	}
	fmt.Println("### RÉSUMÉ FUSIONNÉ ###")
	fmt.Println(m.View())

	// Retour, puis un millésime seul : plus de fusion, plus d'évolution.
	m = envoyer(m, touche("esc"), touche("down"), touche("down"), touche("enter"))
	got = m.(model)
	if got.agrege || got.data.Annee != "2023" || len(got.data.Sales) != 2 {
		t.Fatalf("millésime 2023 attendu, obtenu %v %q %d",
			got.agrege, got.data.Annee, len(got.data.Sales))
	}
	if strings.Contains(sansCouleur(m.View()), "ÉVOLUTION") {
		t.Error("un millésime seul n'a pas d'évolution à montrer")
	}
	for _, o := range got.onglets {
		if o == "2023" || o == "2024" || strings.Contains(o, "Toutes") {
			t.Errorf("la période n'est plus un onglet, obtenus %v", got.onglets)
		}
	}
}
