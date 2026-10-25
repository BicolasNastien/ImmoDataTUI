package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestURLVues(t *testing.T) {
	geo := Sale{
		Numero: "12", Voie: "AV DU PORT", Commune: "Boucau", CodePostal: "64140",
		Lignes: []Row{{Latitude: 43.527, Longitude: -1.4712}},
	}

	// Carte : un repère sur le point, c'est ce qui désigne le bien.
	if got := urlCarte(geo); !strings.Contains(got, "maps/search") ||
		!strings.Contains(got, "43.527000%2C-1.471200") {
		t.Errorf("repère aux coordonnées attendu, obtenu %q", got)
	}
	// Cadastre : Géoportail attend la longitude avant la latitude.
	if got := urlCadastre(geo); !strings.Contains(got, "c=-1.471200,43.527000") ||
		!strings.Contains(got, "CADASTRALPARCELS") {
		t.Errorf("parcelles cadastrales attendues, obtenu %q", got)
	}
	if got := urlRue(geo); !strings.Contains(got, "map_action=pano") ||
		!strings.Contains(got, "43.527000,-1.471200") {
		t.Errorf("vue de rue attendue aux coordonnées, obtenue %q", got)
	}

	// Sans coordonnées : repli sur l'adresse, et pas de cadastre possible.
	sans := Sale{Numero: "12", Voie: "AV DU PORT", Commune: "Boucau", CodePostal: "64140",
		Lignes: []Row{{}}}
	if got := urlCarte(sans); !strings.Contains(got, "12+AV+DU+PORT+64140+Boucau") {
		t.Errorf("recherche sur l'adresse attendue, obtenue %q", got)
	}
	if got := urlCadastre(sans); got != "" {
		t.Errorf("pas de cadastre sans coordonnées, obtenu %q", got)
	}

	if _, ok := VuePourTouche("v"); !ok {
		t.Error("la touche v devrait désigner une vue")
	}
	if _, ok := VuePourTouche("z"); ok {
		t.Error("z ne désigne aucune vue")
	}
}

// TestToucheVue vérifie que « v » ouvre bien la vente sélectionnée, depuis le
// tableau comme depuis la fiche. Le lanceur système est intercepté : aucun
// navigateur n'est ouvert pendant les tests.
func TestToucheVue(t *testing.T) {
	ouverts := make(chan string, 4)
	precedent := ouvrirURL
	ouvrirURL = func(lien string) error { ouverts <- lien; return nil }
	t.Cleanup(func() { ouvrirURL = precedent })

	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,-1.4712,43.527",
		"2024-2,2024-05-02,Vente,310000,4,RUE DES LILAS,64140,64140,Boucau,Maison,120,5,641400000AB2,S,800,,",
	)

	var m tea.Model = NouveauModele(racine, nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: Zone{Code: "64140", Nom: "Boucau"}, annees: []string{"2024"}},
		touche("enter"), // millésime -> stats
		touche("tab"),   // Résumé -> Maisons
	)

	// Tri par date : la vente du 02/05, sans coordonnées, est en tête.
	// Depuis le tableau : la commande part pour la vente sous le curseur.
	m2, cmd := m.Update(touche("v"))
	if cmd == nil {
		t.Fatal("v devrait produire une commande d'ouverture")
	}
	m = envoyer(m2, cmd())
	select {
	case lien := <-ouverts:
		if !strings.Contains(lien, "RUE+DES+LILAS") {
			t.Errorf("recherche sur l'adresse attendue, obtenu %q", lien)
		}
	case <-time.After(time.Second):
		t.Fatal("aucune ouverture reçue")
	}
	if !strings.Contains(sansCouleur(m.View()), "Vue ouverte dans le navigateur") {
		t.Error("l'ouverture devrait être confirmée à l'écran")
	}

	// Depuis la fiche de la vente suivante, géolocalisée : vue de la rue.
	m = envoyer(m, touche("down"), touche("enter"))
	if m.(model).ecran != ecranVente {
		t.Fatal("fiche attendue")
	}
	fiche := sansCouleur(m.(model).contenuVente())
	for _, attendu := range []string{"VOIR LE BIEN", "carte avec repère",
		"parcelles cadastrales", "vue de la rue"} {
		if !strings.Contains(fiche, attendu) {
			t.Errorf("la fiche devrait proposer %q :\n%s", attendu, fiche)
		}
	}

	// c ouvre le cadastre, où la parcelle vendue est dessinée.
	m2, cmd = m.Update(touche("c"))
	if cmd == nil {
		t.Fatal("c devrait ouvrir le cadastre")
	}
	m = envoyer(m2, cmd())
	select {
	case lien := <-ouverts:
		if !strings.Contains(lien, "geoportail") || !strings.Contains(lien, "CADASTRALPARCELS") {
			t.Errorf("Géoportail avec les parcelles attendu, obtenu %q", lien)
		}
	case <-time.After(time.Second):
		t.Fatal("aucune ouverture reçue pour le cadastre")
	}
	m2, cmd = m.Update(touche("V"))
	if cmd == nil {
		t.Fatal("V devrait aussi fonctionner depuis la fiche")
	}
	m = envoyer(m2, cmd())
	select {
	case lien := <-ouverts:
		if !strings.Contains(lien, "43.527000") {
			t.Errorf("coordonnées de la vente attendues, obtenu %q", lien)
		}
	case <-time.After(time.Second):
		t.Fatal("aucune ouverture reçue depuis la fiche")
	}
	fmt.Println("### FICHE AVEC LIEN ###")
	fmt.Println(m.View())
}

// TestVueErreurNavigateur : sans environnement graphique, l'échec doit rester
// lisible et le lien copiable.
func TestVueErreurNavigateur(t *testing.T) {
	var m tea.Model = NouveauModele(t.TempDir(), nil)
	m = envoyer(m, msgNavigateur{lien: "https://exemple/x", err: fmt.Errorf("xdg-open introuvable")})
	note := m.(model).noteVue
	if !strings.Contains(note, "xdg-open introuvable") || !strings.Contains(note, "https://exemple/x") {
		t.Errorf("l'erreur devrait rester lisible avec son lien, obtenu %q", note)
	}
}
