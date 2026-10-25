package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// communesFactices reprend des contours de départements embarqués sous des noms
// de communes : la carte a besoin de géométrie, le filtre seulement de noms.
func communesFactices(t *testing.T) []Zone {
	t.Helper()
	deps, err := DepartementsDe("75")
	if err != nil {
		t.Fatal(err)
	}
	noms := []struct{ code, nom string }{
		{"64024", "Anglet"}, {"64102", "Bayonne"}, {"64122", "Biarritz"},
		{"64140", "Boucau"}, {"64483", "Saint-Jean-de-Luz"}, {"64485", "Saint-Jean-Pied-de-Port"},
		{"40272", "Saint-Martin-de-Seignanx"}, {"40312", "Tarnos"}, {"64547", "Urrugne"},
		{"64558", "Villefranque"}, {"64016", "Arcangues"}, {"64125", "Bidart"},
	}
	var out []Zone
	for i, d := range deps {
		if i >= len(noms) {
			break
		}
		out = append(out, Zone{Code: noms[i].code, Nom: noms[i].nom, Contours: d.Contours})
	}
	return out
}

// selectionnee renvoie la commune sélectionnée ; selection a un récepteur
// pointeur, qu'une valeur tirée de l'interface ne permet pas d'appeler.
func selectionnee(m tea.Model) (Zone, bool) {
	e := m.(model).carte
	return e.selection()
}

func listeAffichee(m tea.Model) []string {
	e := m.(model).carte
	var out []string
	for _, i := range e.visibles() {
		out = append(out, e.zones[i].Nom)
	}
	return out
}

func taper(m tea.Model, texte string) tea.Model {
	for _, r := range texte {
		if r == ' ' {
			m = envoyer(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			continue
		}
		m = envoyer(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestFiltreCommunes(t *testing.T) {
	communes := communesFactices(t)
	var m tea.Model = NouveauModele(t.TempDir(), nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 30},
		msgZones{niveau: nivCommunes, zones: communes})

	m = envoyer(m, touche("/"))
	if !m.(model).carte.saisitFiltre {
		t.Fatal("« / » devrait ouvrir le filtre au niveau des communes")
	}

	// Ni la casse, ni les accents, ni les tirets ne comptent. Les lettres qui
	// sont aussi des raccourcis (q, a, l, r) vont au filtre.
	m = taper(m, "SAINT jean")
	if got := listeAffichee(m); len(got) != 2 || got[0] != "Saint-Jean-de-Luz" {
		t.Fatalf("liste filtrée inattendue : %v", got)
	}
	if m.(model).ecran != ecranCarte {
		t.Fatal("la saisie ne doit déclencher aucun raccourci")
	}
	if vue := sansCouleur(m.View()); !strings.Contains(vue, "/ SAINT jean") || !strings.Contains(vue, "2 communes") {
		t.Errorf("le filtre devrait s'afficher dans le titre :\n%s", vue)
	}

	// ↓ se déplace dans la liste filtrée, pas dans le département entier.
	m = envoyer(m, touche("down"))
	if z, _ := selectionnee(m); z.Nom != "Saint-Jean-Pied-de-Port" {
		t.Errorf("sélection %q après ↓", z.Nom)
	}
	m = envoyer(m, touche("down"))
	if z, _ := selectionnee(m); z.Nom != "Saint-Jean-Pied-de-Port" {
		t.Errorf("↓ en bout de liste filtrée devrait rester sur place, obtenu %q", z.Nom)
	}

	// Effacer reprend la liste plus large.
	for range "jean" {
		m = envoyer(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = envoyer(m, tea.KeyMsg{Type: tea.KeyBackspace}) // l'espace
	if got := listeAffichee(m); len(got) != 3 {
		t.Errorf("« SAINT » devrait donner trois communes, obtenu %v", got)
	}

	// Aucune correspondance : ↵ n'ouvre rien.
	m = taper(m, "zzz")
	if _, ok := selectionnee(m); ok {
		t.Error("aucune commune ne correspond, aucune ne devrait être sélectionnée")
	}
	m = envoyer(m, touche("enter"))
	if m.(model).ecran != ecranCarte {
		t.Error("↵ sans correspondance ne devrait rien ouvrir")
	}

	// esc efface tout et rend la liste complète.
	m = envoyer(m, touche("/"), touche("esc"))
	if e := m.(model).carte; e.filtreActif() || len(listeAffichee(m)) != len(communes) {
		t.Errorf("filtre effacé attendu, obtenu %q (%d communes)", e.filtre, len(listeAffichee(m)))
	}
}

// TestFiltreEtOuverture : ↵ ouvre la commune trouvée, et le filtre retrouvé
// au retour se retire avant de remonter d'un échelon.
func TestFiltreEtOuverture(t *testing.T) {
	var m tea.Model = NouveauModele(t.TempDir(), nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 30},
		msgZones{niveau: nivCommunes, zones: communesFactices(t)})

	// Le code INSEE marche aussi bien que le nom.
	m = envoyer(m, touche("/"))
	m = taper(m, "64140")
	if got := listeAffichee(m); len(got) != 1 || got[0] != "Boucau" {
		t.Fatalf("recherche par code INSEE : %v", got)
	}

	m, cmd := m.Update(touche("enter"))
	if m.(model).ecran != ecranAttente || cmd == nil {
		t.Fatal("↵ devrait lancer l'extraction de la commune filtrée")
	}
	if m.(model).carte.saisitFiltre {
		t.Error("le clavier devrait être rendu après l'ouverture")
	}

	// Retour sur la carte : le filtre est toujours là, esc le retire d'abord.
	mm := m.(model)
	mm.ecran = ecranCarte
	m = envoyer(mm, touche("esc"))
	if m.(model).ecran != ecranCarte || m.(model).carte.filtre != "" {
		t.Error("esc devrait d'abord effacer le filtre, sans quitter le département")
	}
	if z, _ := selectionnee(m); z.Nom != "Boucau" {
		t.Errorf("la commune choisie devrait rester sélectionnée, obtenu %q", z.Nom)
	}

	// Au niveau des départements, « / » reste la recherche d'adresse.
	deps, _ := DepartementsDe("75")
	m = envoyer(m, msgZones{niveau: nivDepartements, zones: deps}, touche("/"))
	if m.(model).ecran != ecranRecherche {
		t.Error("« / » sur les départements devrait ouvrir la recherche d'adresse")
	}
}

func TestPrefixeAvantContenu(t *testing.T) {
	e := etatCarte{zones: []Zone{
		{Code: "1", Nom: "Saint-Martin-de-Boulogne"},
		{Code: "2", Nom: "Boucau"},
		{Code: "3", Nom: "Bourg"},
	}, filtre: "bou"}
	vis := e.visibles()
	if len(vis) != 3 || e.zones[vis[0]].Nom != "Boucau" || e.zones[vis[2]].Nom != "Saint-Martin-de-Boulogne" {
		t.Errorf("ordre inattendu : %v", vis)
	}
}
