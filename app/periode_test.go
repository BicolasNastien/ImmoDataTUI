package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestAdressePuisMillesime : la page d'adresse décrit toute la période, même
// quand les onglets de type de bien sont revenus sur un millésime unique.
func TestAdressePuisMillesime(t *testing.T) {
	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2023", "64140",
		"2023-1,2023-04-11,Vente,240000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,")
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,")

	a := Adresse{Label: "12 Avenue du Port 64140 Boucau", Numero: "12", Voie: "Avenue du Port",
		Commune: "Boucau", CodeInsee: "64140", Type: "housenumber"}
	mm := NouveauModele(racine, nil)
	mm.adresse = &a
	mm.commune = Zone{Code: "64140", Nom: "Boucau"}
	var m tea.Model = mm
	m = envoyer(m, tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: mm.commune, annees: []string{"2024", "2023"}})
	if n := len(m.(model).ventesOnglet()); n != 2 {
		t.Fatalf("2 ventes dans la voie attendues, obtenues %d", n)
	}

	// L'utilisateur revient choisir un millésime, puis retourne sur l'adresse.
	m = envoyer(m, touche("esc"), touche("down"), touche("down"), touche("enter"))
	got := m.(model)
	if got.agrege || got.data.Annee != "2023" {
		t.Fatalf("millésime 2023 attendu pour les onglets de type, obtenu %q", got.data.Annee)
	}
	// L'onglet Adresse, lui, reste sur toute la période.
	if len(got.total.Sales) != 2 || len(got.ventesOnglet()) != 2 {
		t.Fatalf("la page d'adresse devrait rester agrégée, obtenu total=%d ventes=%d",
			len(got.total.Sales), len(got.ventesOnglet()))
	}
	vue := sansCouleur(m.View())
	for _, attendu := range []string{"12 Avenue du Port", "2023 – 2024", "240 000 €", "265 000 €"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("la page d'adresse devrait contenir %q", attendu)
		}
	}
	fmt.Println("### ADRESSE APRÈS CHOIX D'UN MILLÉSIME ###")
	fmt.Println(m.View())
}

// TestRetourCarteApresAdresse : après une recherche d'adresse, la carte doit
// redevenir une carte — choisir un département ramène à ses communes, pas aux
// statistiques de la commune précédente.
func TestRetourCarteApresAdresse(t *testing.T) {
	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,")

	a := Adresse{Label: "12 Avenue du Port 64140 Boucau", Numero: "12", Voie: "Avenue du Port",
		Commune: "Boucau", CodeInsee: "64140", Type: "housenumber"}
	mm := NouveauModele(racine, nil)
	mm.adresse = &a
	mm.commune = Zone{Code: "64140", Nom: "Boucau"}

	var m tea.Model = mm
	m = envoyer(m, tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: mm.commune, annees: []string{"2024"}})
	if m.(model).ecran != ecranStats {
		t.Fatal("page d'adresse attendue")
	}

	// Retour à la carte, puis choix d'un département.
	deps, err := DepartementsDe("75")
	if err != nil {
		t.Fatal(err)
	}
	m = envoyer(m, touche("esc"), touche("esc"),
		msgZones{niveau: nivDepartements, zones: deps},
		touche("enter"))
	got := m.(model)
	if got.ecran != ecranTelechargement {
		t.Fatalf("téléchargement du département attendu, écran %v", got.ecran)
	}
	if got.adresse != nil {
		t.Error("l'adresse précédente ne devrait plus être en cours")
	}
	if len(got.onglets) != len(ongletsStandard) || got.onglets[0] == "Adresse" {
		t.Errorf("onglets remis à leur état normal attendus, obtenus %v", got.onglets)
	}

	// Millésimes reçus : la carte des communes doit être demandée.
	for _, annee := range got.tele.annees {
		m = envoyer(m, msgTelechargement(ResultatTelechargement{
			Dep: got.dep.Code, Annee: annee, Octets: 1 << 20}))
	}
	fin := m.(model)
	if fin.ecran != ecranAttente || !strings.Contains(fin.attente, "communes") {
		t.Fatalf("contours des communes attendus, obtenu écran %v (%q)", fin.ecran, fin.attente)
	}
}

// TestCarteCommunesPerimee : une recherche d'adresse peut changer de
// département sans passer par la carte. Celle qui reste affichée décrit alors
// un autre département — elle doit être redemandée, pas réutilisée.
func TestCarteCommunesPerimee(t *testing.T) {
	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,,")

	// On explore les communes du 10 quand une adresse du 64 est ouverte.
	a := Adresse{Label: "12 Avenue du Port 64140 Boucau", Numero: "12", Voie: "Avenue du Port",
		Commune: "Boucau", CodeInsee: "64140", Type: "housenumber"}
	mm := NouveauModele(racine, nil)
	mm.carte = etatCarte{niveau: nivCommunes, dep: "10", zones: []Zone{{Code: "10387", Nom: "Troyes"}}}
	mm.dep = Zone{Code: "64", Nom: "Pyrénées-Atlantiques"}
	mm.adresse = &a
	mm.commune = Zone{Code: "64140", Nom: "Boucau"}

	var m tea.Model = mm
	m = envoyer(m, tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: mm.commune, annees: []string{"2024"}},
		touche("esc"), touche("esc"))

	got := m.(model)
	if got.ecran != ecranAttente || !strings.Contains(got.attente, "64") {
		t.Fatalf("les communes du 64 devraient être redemandées, obtenu écran %v (%q)",
			got.ecran, got.attente)
	}
}
