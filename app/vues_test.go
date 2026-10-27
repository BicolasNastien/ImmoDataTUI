package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSuiviEntreDeuxSessions : « nouvelle » se mesure par rapport au
// lancement du programme, « vue » survit d'une session à l'autre.
func TestSuiviEntreDeuxSessions(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "immodata", "annonces.json")
	lundi := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	a := Annonce{Source: "Bien'ici", ID: "a"}
	b := Annonce{Source: "Bien'ici", ID: "b"}

	s, err := ChargerSuivi(chemin, lundi)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Croiser([]Annonce{a, b}, lundi.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Relancer la même recherche dans la même session ne doit rien effacer.
	if err := s.Croiser([]Annonce{a, b}, lundi.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !s.Nouvelle(a) || !s.Nouvelle(b) {
		t.Error("les annonces croisées pendant la session restent nouvelles")
	}
	if err := s.MarquerVue(a, lundi.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if s.Marque(a) != "✓" || s.Marque(b) != "●" {
		t.Errorf("marques : a=%q b=%q", s.Marque(a), s.Marque(b))
	}

	mardi := lundi.Add(24 * time.Hour)
	s, err = ChargerSuivi(chemin, mardi)
	if err != nil {
		t.Fatal(err)
	}
	c := Annonce{Source: "Bien'ici", ID: "c"}
	if err := s.Croiser([]Annonce{a, b, c}, mardi.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if s.Nouvelle(b) {
		t.Error("une annonce croisée la veille n'est plus nouvelle")
	}
	if !s.Nouvelle(c) {
		t.Error("une annonce jamais croisée est nouvelle")
	}
	if !s.Vue(a) {
		t.Error("la marque « vue » doit survivre au redémarrage")
	}
	if n, v := s.Compter([]Annonce{a, b, c}); n != 1 || v != 1 {
		t.Errorf("Compter = %d nouvelles, %d vues ; attendu 1 et 1", n, v)
	}

	// Même identifiant, autre portail : ce n'est pas la même annonce.
	if s.Vue(Annonce{Source: "Autre", ID: "a"}) {
		t.Error("la clé de suivi doit inclure la source")
	}
}

func TestSuiviBasculer(t *testing.T) {
	s := nouveauSuivi("", time.Now())
	a := Annonce{Source: "Bien'ici", ID: "a"}
	if vue, _ := s.Basculer(a, time.Now()); !vue || !s.Vue(a) {
		t.Error("une première bascule marque l'annonce vue")
	}
	if vue, _ := s.Basculer(a, time.Now()); vue || s.Vue(a) {
		t.Error("une seconde bascule retire la marque")
	}
}

func TestSuiviOublieLesVieillesAnnonces(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "annonces.json")
	avant := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s, _ := ChargerSuivi(chemin, avant)
	a := Annonce{Source: "Bien'ici", ID: "a"}
	_ = s.Croiser([]Annonce{a}, avant)
	_ = s.MarquerVue(a, avant)

	s, err := ChargerSuivi(chemin, avant.Add(dureeSuivi+time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Croisees) != 0 || len(s.Vues) != 0 {
		t.Errorf("entrées de plus d'un an conservées : %v %v", s.Croisees, s.Vues)
	}
}

// Un fichier abîmé ne doit pas empêcher de chercher des annonces.
func TestSuiviFichierIllisible(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "annonces.json")
	if err := os.WriteFile(chemin, []byte("{pas du json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ChargerSuivi(chemin, time.Now())
	if err == nil {
		t.Error("l'erreur de lecture doit être signalée")
	}
	if s == nil || s.Croisees == nil || s.Vues == nil {
		t.Fatal("le suivi renvoyé doit rester utilisable")
	}
	if err := s.MarquerVue(Annonce{Source: "x", ID: "y"}, time.Now()); err != nil {
		t.Errorf("le suivi doit pouvoir réécrire le fichier : %v", err)
	}
}

// TestResultatsMarquesEtPublication : écran des résultats, de bout en bout.
func TestResultatsMarquesEtPublication(t *testing.T) {
	ouverts := make(chan string, 1)
	precedent := ouvrirURL
	ouvrirURL = func(lien string) error { ouverts <- lien; return nil }
	t.Cleanup(func() { ouvrirURL = precedent })

	recente := Annonce{Source: "Bien'ici", ID: "r", URL: "https://exemple.fr/r", Type: BienMaison,
		Prix: 300000, Publiee: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)}
	masquee := Annonce{Source: "Bien'ici", ID: "m", URL: "https://exemple.fr/m", Type: BienMaison,
		Prix: 250000, PublieeAvant: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)}

	var m tea.Model = NouveauModele(t.TempDir(), nil)
	if tri := m.(model).triAnn; tri.Critere != TriADate || !tri.Decroissant {
		t.Fatalf("tri par défaut = %v, attendu publication décroissante", tri)
	}
	mm := m.(model)
	mm.ecran = ecranResultats
	m = envoyer(mm, msgAnnonces{res: ResultatAnnonces{Annonces: []Annonce{recente, masquee}}})

	vue := sansCouleur(m.(model).vueResultats())
	for _, attendu := range []string{"2 nouvelles", "0 vue", "20/10/26", "<28/08/26", "●"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("%q absent des résultats :\n%s", attendu, vue)
		}
	}
	if strings.Contains(vue, "1970") {
		t.Error("une date masquée ne doit jamais s'afficher en 1970")
	}

	// ↵ ouvre l'annonce et la marque vue ; « m » retire la marque.
	m, cmd := m.Update(touche("enter"))
	if cmd == nil {
		t.Fatal("↵ devrait ouvrir l'annonce")
	}
	cmd()
	if lien := <-ouverts; lien != recente.URL {
		t.Errorf("lien ouvert : %q", lien)
	}
	if !m.(model).suivi.Vue(recente) {
		t.Error("une annonce ouverte doit être marquée vue")
	}
	if !strings.Contains(sansCouleur(m.(model).vueResultats()), "1 vue") {
		t.Error("le titre doit compter l'annonce vue")
	}
	m = envoyer(m, touche("m"))
	if m.(model).suivi.Vue(recente) {
		t.Error("« m » doit retirer la marque")
	}
}
