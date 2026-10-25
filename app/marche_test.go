package main

import (
	"context"
	"math"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// maison fabrique une vente DVF de maison au prix au m² donné.
func maison(surface float64, pieces int, terrain, prixM2 float64) Sale {
	return Sale{Type: "Maison", SurfaceBati: surface, Pieces: pieces, SurfaceTerrain: terrain,
		Valeur: surface * prixM2, Commune: "Boucau", Lignes: []Row{{CodeCommune: "64140"}}}
}

func repeter(n int, s Sale) []Sale {
	out := make([]Sale, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func proche(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestComparerRetientLesBiensSimilaires(t *testing.T) {
	var ventes []Sale
	ventes = append(ventes, repeter(5, maison(90, 4, 600, 3000))...)
	// Autant de ventes qui ne ressemblent pas à l'annonce, et beaucoup plus
	// chères : si elles entraient dans la référence, l'écart serait négatif.
	ventes = append(ventes, repeter(5, maison(200, 7, 3000, 6000))...) // trop grandes
	ventes = append(ventes, repeter(5, maison(90, 2, 600, 6000))...)   // deux pièces
	ventes = append(ventes, repeter(5, maison(90, 4, 5000, 6000))...)  // grand terrain
	ventes = append(ventes, repeter(5, Sale{Type: "Appartement", SurfaceBati: 90, Pieces: 4, Valeur: 90 * 6000})...)

	a := Annonce{Type: BienMaison, Prix: 300000, Surface: 92, Pieces: 4, SurfaceTerrain: 500}
	c := Comparer(a, ventes)
	if c.Niveau != SimilariteStricte || len(c.Similaires) != 5 {
		t.Fatalf("5 similaires stricts attendus, obtenu niveau %v, %d", c.Niveau, len(c.Similaires))
	}
	if !proche(c.PrixM2Vendu, 3000) || !proche(c.Estime, 276000) {
		t.Errorf("référence inattendue : %.0f €/m², estimé %.0f", c.PrixM2Vendu, c.Estime)
	}
	if !proche(c.Ecart, (300000.0/276000-1)*100) {
		t.Errorf("écart %.2f %%", c.Ecart)
	}
}

func TestComparerElargitFauteDeVentes(t *testing.T) {
	// Cinq pièces et 105 m² : hors du critère strict (une pièce et 17 % de
	// surface d'écart), dans le critère large.
	ventes := repeter(5, maison(105, 5, 600, 3000))
	a := Annonce{Type: BienMaison, Prix: 300000, Surface: 90, Pieces: 4, SurfaceTerrain: 600}

	c := Comparer(a, ventes)
	if c.Niveau != SimilariteElargie {
		t.Fatalf("critère élargi attendu, obtenu %v", c.Niveau)
	}

	// Une vente de moins et plus rien : pas de médiane sur quatre biens.
	if c := Comparer(a, ventes[:4]); c.Comparable() {
		t.Errorf("4 ventes ne devraient pas suffire, obtenu %v", c.Niveau)
	}
}

func TestComparerChampsTaisParLAnnonce(t *testing.T) {
	ventes := repeter(5, maison(90, 4, 600, 3000))

	// Ni pièces ni terrain : seuls le type et la surface contraignent.
	if c := Comparer(Annonce{Type: BienMaison, Prix: 270000, Surface: 90}, ventes); c.Niveau != SimilariteStricte {
		t.Errorf("une annonce muette sur les pièces reste comparable, obtenu %v", c.Niveau)
	}
	// Sans surface, aucune estimation n'est possible.
	if c := Comparer(Annonce{Type: BienMaison, Prix: 270000, Pieces: 4}, ventes); c.Comparable() {
		t.Error("une annonce sans surface ne peut pas être comparée")
	}
	// Une annonce avec terrain ne se compare pas à des maisons sans terrain.
	sansTerrain := repeter(5, maison(90, 4, 0, 3000))
	if c := Comparer(Annonce{Type: BienMaison, Prix: 270000, Surface: 90, SurfaceTerrain: 400}, sansTerrain); c.Comparable() {
		t.Error("terrain annoncé, ventes sans terrain : pas comparable")
	}
}

func TestComparerTerrain(t *testing.T) {
	ventes := repeter(5, Sale{Type: "Terrain", SurfaceTerrain: 500, Valeur: 100000})
	c := Comparer(Annonce{Type: BienTerrain, Prix: 110000, SurfaceTerrain: 520}, ventes)
	if !c.Comparable() || !proche(c.Estime, 104000) {
		t.Fatalf("terrain : %v, estimé %.0f", c.Niveau, c.Estime)
	}
}

func TestComparerToutEtSynthese(t *testing.T) {
	ventes := repeter(5, maison(90, 4, 600, 3000))
	as := []Annonce{
		{ID: "cher", Type: BienMaison, Prix: 297000, Surface: 90, Pieces: 4}, // +10 %
		{ID: "isolee", Type: BienMaison, Prix: 900000, Surface: 400},         // aucune similaire
		{ID: "bon", Type: BienMaison, Prix: 243000, Surface: 90, Pieces: 4},  // -10 %
	}
	cs := ComparerTout(as, ventes)
	if cs[0].Annonce.ID != "bon" || cs[1].Annonce.ID != "cher" || cs[2].Annonce.ID != "isolee" {
		t.Fatalf("ordre inattendu : %s %s %s", cs[0].Annonce.ID, cs[1].Annonce.ID, cs[2].Annonce.ID)
	}

	synth := Synthetiser(cs)
	if len(synth) != 1 {
		t.Fatalf("une synthèse (maisons) attendue, obtenu %+v", synth)
	}
	s := synth[0]
	if s.Annonces != 3 || s.Comparees != 2 || s.AuDessus != 1 || !proche(s.EcartMed, 0) {
		t.Errorf("synthèse inattendue : %+v", s)
	}
	if !proche(s.PrixM2Vendu, 3000) || !proche(s.PrixM2Ann, 3000) {
		t.Errorf("€/m² : demandé %.0f, vendu %.0f", s.PrixM2Ann, s.PrixM2Vendu)
	}
}

func TestPreparerMarche(t *testing.T) {
	as := []Annonce{
		{ID: "a", Type: BienMaison, Prix: 300000, Surface: 90, Pieces: 4},
		{ID: "b", Type: BienMaison, Prix: 300000, Surface: 90, Pieces: 4}, // même bien, autre agence
		{ID: "c", Type: BienMaison, Prix: 300000, Surface: 90, Pieces: 5},
		{ID: "d", Type: BienMaison, Prix: 140000, Surface: 104, Viager: true},
	}
	gardees, viagers, doublons := PreparerMarche(as)
	if len(gardees) != 2 || viagers != 1 || doublons != 1 {
		t.Fatalf("gardées %d, viagers %d, doublons %d", len(gardees), viagers, doublons)
	}
}

func TestComparerPrixDeLActe(t *testing.T) {
	ventes := repeter(5, maison(90, 4, 600, 3000))
	// 283 500 € affichés, dont 13 500 € d'honoraires à la charge de l'acheteur :
	// l'acte portera 270 000 €, exactement le prix des ventes similaires.
	a := Annonce{Type: BienMaison, Prix: 283500, PrixActe: 270000, Surface: 90, Pieces: 4}
	if c := Comparer(a, ventes); !proche(c.Ecart, 0) {
		t.Errorf("écart %.1f %% : les honoraires ne devraient pas compter", c.Ecart)
	}
	a.Viager = true
	if c := Comparer(a, ventes); c.Comparable() {
		t.Error("un viager ne se compare pas")
	}
}

func TestCommuneDominante(t *testing.T) {
	ventes := []Sale{
		{Commune: "Boucau", Lignes: []Row{{CodeCommune: "64140"}}},
		{Commune: "Boucau", Lignes: []Row{{CodeCommune: "64140"}}},
		{Commune: "Tarnos", Lignes: []Row{{CodeCommune: "40312"}}},
		{},
	}
	if code, nom := CommuneDominante(ventes); code != "64140" || nom != "Boucau" {
		t.Errorf("obtenu %q %q", code, nom)
	}
	if code, _ := CommuneDominante(nil); code != "" {
		t.Errorf("aucune vente, aucun code : %q", code)
	}
}

func TestZonesDeCommune(t *testing.T) {
	zones := []ZoneAnnonce{
		{Source: "A", ID: "dep", Niveau: ZoneDepartement, CodeInsee: "64140"},
		{Source: "A", ID: "homonyme", Niveau: ZoneCommune, CodeInsee: "33140"},
		{Source: "A", ID: "bonne", Niveau: ZoneCommune, CodeInsee: "64140"},
		{Source: "A", ID: "doublon", Niveau: ZoneCommune, CodeInsee: "64140"},
		{Source: "B", ID: "autre-portail", Niveau: ZoneCommune, CodeInsee: "64140"},
	}
	got := zonesDeCommune(zones, "64140")
	if len(got) != 2 || got[0].ID != "bonne" || got[1].ID != "autre-portail" {
		t.Errorf("obtenu %+v", got)
	}
}

// portailCommune : un portail factice qui connaît une commune par son code.
type portailCommune struct {
	insee    string
	annonces []Annonce
	recu     *Criteres
}

func (portailCommune) Nom() string { return "Factice" }

func (p portailCommune) ResoudreZone(context.Context, string) ([]ZoneAnnonce, error) {
	return []ZoneAnnonce{{Source: "Factice", ID: "z", Nom: "Boucau", Niveau: ZoneCommune, CodeInsee: p.insee}}, nil
}

func (p portailCommune) Chercher(_ context.Context, c Criteres) ([]Annonce, int, error) {
	*p.recu = c
	return p.annonces, len(p.annonces), nil
}

// TestOngletMarche suit l'onglet de bout en bout, réseau excepté : la commande
// produite par la touche est exécutée à la main, et son message réinjecté.
func TestOngletMarche(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	var lignes []string
	for i := 0; i < 6; i++ {
		lignes = append(lignes, "2024-"+string(rune('1'+i))+",2024-03-12,Vente,270000,RUE DES LILAS,64140,Boucau,Maison,90,4,641400000AB1,S,600")
	}
	archiveFactice(t, "2024", "64", lignes...)
	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}

	var recu Criteres
	avecSources(t, portailCommune{insee: "64140", recu: &recu, annonces: []Annonce{
		{Source: "Factice", ID: "1", URL: "https://exemple.fr/1", Titre: "Maison avec jardin", Type: BienMaison,
			Prix: 330000, Surface: 92, Pieces: 4, SurfaceTerrain: 550},
		{Source: "Factice", ID: "2", URL: "https://exemple.fr/2", Titre: "Grande propriété", Type: BienMaison,
			Prix: 950000, Surface: 300, Pieces: 9},
	}})

	var m tea.Model = NouveauModele(racine, nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 50},
		msgExtraction{commune: Zone{Code: "64140", Nom: "Boucau"}, annees: annees},
		touche("enter"))

	// Les onglets ordinaires ne sollicitent aucun portail.
	var cmd tea.Cmd
	for m.(model).onglets[m.(model).curOnglet] != ongletMarche {
		if cmd != nil {
			t.Fatal("un onglet DVF ne devrait pas interroger les portails")
		}
		m, cmd = m.Update(touche("tab"))
	}
	if cmd == nil {
		t.Fatal("l'onglet « En vente » devrait lancer la recherche")
	}
	if !strings.Contains(sansCouleur(m.View()), "Recherche des biens en vente") {
		t.Error("l'attente devrait être signalée")
	}

	m = envoyer(m, cmd())
	if !recu.ExclureNeuf || len(recu.Zones) != 1 || recu.Zones[0].CodeInsee != "64140" {
		t.Errorf("critères envoyés inattendus : %+v", recu)
	}

	mm := m.(model)
	vue := sansCouleur(mm.vueMarche())
	for _, attendu := range []string{"PRIX DEMANDÉ CONTRE PRIX DE VENTE", "Maison avec jardin", "+20 %",
		"1 annonce sans comparaison"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("%q absent de l'onglet :\n%s", attendu, vue)
		}
	}

	// ↑/↓ parcourent les annonces, ↵ ouvre celle qui est sélectionnée. La
	// première est la comparée, l'autre vient ensuite.
	ouverts := make(chan string, 1)
	precedent := ouvrirURL
	ouvrirURL = func(lien string) error { ouverts <- lien; return nil }
	t.Cleanup(func() { ouvrirURL = precedent })

	if a, _ := m.(model).annonceMarche(); a.ID != "1" {
		t.Fatalf("première annonce sélectionnée attendue, obtenu %q", a.ID)
	}
	m = envoyer(m, touche("down"))
	if a, _ := m.(model).annonceMarche(); a.ID != "2" {
		t.Fatalf("↓ devrait sélectionner l'annonce sans comparaison, obtenu %q", a.ID)
	}
	if !strings.Contains(sansCouleur(m.View()), "▸") {
		t.Error("la ligne sélectionnée devrait être marquée")
	}
	m, cmd = m.Update(touche("enter"))
	if cmd == nil {
		t.Fatal("↵ devrait ouvrir l'annonce")
	}
	m = envoyer(m, cmd())
	if lien := <-ouverts; lien != "https://exemple.fr/2" {
		t.Errorf("lien ouvert : %q", lien)
	}
	if m.(model).ecran != ecranStats {
		t.Error("ouvrir une annonce ne doit pas quitter l'onglet")
	}

	// Revenir sur l'onglet ne relance rien : les annonces sont gardées.
	m, _ = m.Update(touche("tab"))
	if _, cmd = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab}); cmd != nil {
		t.Error("les annonces de la commune devraient être réutilisées")
	}
}
