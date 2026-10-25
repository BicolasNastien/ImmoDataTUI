package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestDistanceKm(t *testing.T) {
	// Paris (Notre-Dame) → Lyon (Bellecour) : 392 km à quelques centaines de
	// mètres près. Une valeur de référence extérieure au programme, sinon le
	// test ne vérifie que sa propre arithmétique.
	got := DistanceKm(48.8530, 2.3499, 45.7578, 4.8320)
	if math.Abs(got-392) > 3 {
		t.Errorf("Paris–Lyon = %.1f km, attendu ~392 km", got)
	}
	if d := DistanceKm(43.49, -1.47, 43.49, -1.47); d != 0 {
		t.Errorf("distance d'un point à lui-même = %v, attendu 0", d)
	}
}

func TestCorrespond(t *testing.T) {
	maison := Annonce{
		Type: BienMaison, Prix: 250000, Surface: 100, SurfaceTerrain: 600,
		Pieces: 4, Lat: 43.49, Lon: -1.47,
	}
	terrain := Annonce{Type: BienTerrain, Prix: 90000, SurfaceTerrain: 800, Lat: 43.49, Lon: -1.47}

	cas := []struct {
		nom  string
		a    Annonce
		c    Criteres
		veut bool
	}{
		{"sans critère", maison, Criteres{}, true},
		{"type exclu", maison, Criteres{Types: []TypeBien{BienAppartement}}, false},
		{"prix dans les bornes", maison, Criteres{PrixMin: 200000, PrixMax: 300000}, true},
		{"prix trop haut", maison, Criteres{PrixMax: 200000}, false},
		{"surface max", maison, Criteres{SurfaceMax: 90}, false},
		{"terrain min", maison, Criteres{TerrainMin: 700}, false},
		{"pièces min", maison, Criteres{PiecesMin: 5}, false},

		// Un champ absent ne peut pas être jugé : un terrain nu n'a pas de
		// surface habitable, l'écarter sur ce critère serait un contresens.
		{"surface absente non jugée", terrain, Criteres{SurfaceMin: 50}, true},
		{"pièces absentes non jugées", terrain, Criteres{PiecesMin: 3}, true},

		{"neuf exclu", Annonce{Type: BienMaison, Neuf: true}, Criteres{ExclureNeuf: true}, false},

		{"dans le rayon", maison,
			Criteres{CentreLat: 43.49, CentreLon: -1.47, RayonKm: 5}, true},
		{"hors du rayon", maison,
			Criteres{CentreLat: 48.85, CentreLon: 2.35, RayonKm: 5}, false},
		// Sans position, on ne peut pas affirmer que le bien est dans le
		// cercle : on l'écarte plutôt que de le promettre.
		{"non localisée avec rayon", Annonce{Type: BienMaison, Prix: 1},
			Criteres{CentreLat: 43.49, CentreLon: -1.47, RayonKm: 5}, false},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := Correspond(c.a, c.c); got != c.veut {
				t.Errorf("Correspond = %v, attendu %v", got, c.veut)
			}
		})
	}
}

func TestTrierAnnoncesValeursAbsentesEnFin(t *testing.T) {
	as := []Annonce{
		{ID: "sans", Prix: 0},
		{ID: "cher", Prix: 500000},
		{ID: "pas-cher", Prix: 100000},
	}

	TrierAnnonces(as, TriAnnonces{Critere: TriAPrix})
	if as[0].ID != "pas-cher" || as[2].ID != "sans" {
		t.Errorf("tri croissant : %s, %s, %s", as[0].ID, as[1].ID, as[2].ID)
	}

	TrierAnnonces(as, TriAnnonces{Critere: TriAPrix, Decroissant: true})
	if as[0].ID != "cher" || as[2].ID != "sans" {
		t.Errorf("tri décroissant : %s, %s, %s", as[0].ID, as[1].ID, as[2].ID)
	}
}

func TestTrierAnnoncesEstDeterministe(t *testing.T) {
	// Deux annonces au même prix doivent sortir deux fois dans le même ordre,
	// sinon le curseur de l'utilisateur saute d'une ligne à l'autre à chaque
	// changement de tri.
	as := []Annonce{{ID: "b", Prix: 100}, {ID: "a", Prix: 100}, {ID: "c", Prix: 100}}
	TrierAnnonces(as, TriAnnonces{Critere: TriAPrix})
	if as[0].ID != "a" || as[1].ID != "b" || as[2].ID != "c" {
		t.Errorf("ordre non déterministe : %s %s %s", as[0].ID, as[1].ID, as[2].ID)
	}
}

func TestTrierAnnoncesPrixM2(t *testing.T) {
	as := []Annonce{
		{ID: "grande", Prix: 300000, Surface: 150}, // 2000 €/m²
		{ID: "petite", Prix: 200000, Surface: 50},  // 4000 €/m²
		{ID: "terrain", Prix: 90000},               // pas de €/m² : en fin de liste
	}
	TrierAnnonces(as, TriAnnonces{Critere: TriAPrixM2})
	if as[0].ID != "grande" || as[2].ID != "terrain" {
		t.Errorf("tri €/m² : %s, %s, %s", as[0].ID, as[1].ID, as[2].ID)
	}
}

func TestFiltrerAnnoncesDeduplique(t *testing.T) {
	as := []Annonce{
		{Source: "A", ID: "1", Type: BienMaison, Prix: 100},
		{Source: "A", ID: "1", Type: BienMaison, Prix: 100}, // republiée
		{Source: "B", ID: "1", Type: BienMaison, Prix: 100}, // autre portail : autre bien
	}
	if got := FiltrerAnnonces(as, Criteres{}); len(got) != 2 {
		t.Errorf("%d annonces après déduplication, attendu 2", len(got))
	}
}

func TestTriAnnoncesLibelles(t *testing.T) {
	// Un critère de tri sans libellé afficherait une colonne muette.
	if len(LibellesTriAnnonces) != int(TriADate)+1 {
		t.Fatalf("%d libellés pour %d critères", len(LibellesTriAnnonces), TriADate+1)
	}
	t1 := TriAnnonces{Critere: TriAPrix}
	if t1.Suivant().Critere != TriAPrixM2 {
		t.Error("Suivant ne passe pas au critère d'après")
	}
	if !t1.Inverse().Decroissant || t1.Inverse().Critere != TriAPrix {
		t.Error("Inverse doit changer le sens sans changer le critère")
	}
}

// ---------------------------------------------------------------------------
// La modularité des sources
// ---------------------------------------------------------------------------

// sourceFactice sert à vérifier l'agrégation sans toucher au réseau. Si écrire
// une source de test demandait plus que ces trois méthodes, c'est que
// l'interface serait trop large.
type sourceFactice struct {
	nom      string
	annonces []Annonce
	total    int
	err      error
	vues     *[]Criteres // ce que la source a reçu, pour vérifier la répartition
}

func (s sourceFactice) Nom() string { return s.nom }

func (s sourceFactice) ResoudreZone(context.Context, string) ([]ZoneAnnonce, error) {
	return []ZoneAnnonce{{Source: s.nom, ID: "z1", Nom: "Zone " + s.nom}}, nil
}

func (s sourceFactice) Chercher(_ context.Context, c Criteres) ([]Annonce, int, error) {
	if s.vues != nil {
		*s.vues = append(*s.vues, c)
	}
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.annonces, s.total, nil
}

// avecSources remplace le registre le temps d'un test et le restaure ensuite.
// Un registre global est commode ; il n'est acceptable que si les tests savent
// le rendre dans l'état où ils l'ont trouvé.
func avecSources(t *testing.T, ss ...SourceAnnonces) {
	t.Helper()
	anciennes, ancienOrdre := sources, ordreSources
	sources, ordreSources = map[string]SourceAnnonces{}, nil
	for _, s := range ss {
		EnregistrerSource(s)
	}
	t.Cleanup(func() { sources, ordreSources = anciennes, ancienOrdre })
}

func TestChercherAnnoncesAgregeEtTrie(t *testing.T) {
	avecSources(t,
		sourceFactice{nom: "A", total: 10, annonces: []Annonce{
			{Source: "A", ID: "a1", Type: BienMaison, Prix: 300000},
		}},
		sourceFactice{nom: "B", total: 5, annonces: []Annonce{
			{Source: "B", ID: "b1", Type: BienMaison, Prix: 100000},
		}},
	)

	res, err := ChercherAnnonces(context.Background(), Criteres{Tri: TriAnnonces{Critere: TriAPrix}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Annonces) != 2 {
		t.Fatalf("%d annonces, attendu 2", len(res.Annonces))
	}
	if res.Annonces[0].ID != "b1" {
		t.Errorf("le tri global doit mélanger les sources, pas les concaténer : %s en tête", res.Annonces[0].ID)
	}
	if res.Total != 15 {
		t.Errorf("total = %d, attendu 15", res.Total)
	}
}

func TestChercherAnnoncesUneSourceEnPanne(t *testing.T) {
	// Une source qui tombe ne doit pas emporter les autres.
	avecSources(t,
		sourceFactice{nom: "A", err: errors.New("503")},
		sourceFactice{nom: "B", total: 1, annonces: []Annonce{
			{Source: "B", ID: "b1", Type: BienMaison, Prix: 100000},
		}},
	)

	res, err := ChercherAnnonces(context.Background(), Criteres{})
	if err != nil {
		t.Fatalf("une panne partielle ne doit pas faire échouer la recherche : %v", err)
	}
	if len(res.Annonces) != 1 {
		t.Errorf("%d annonces, attendu 1", len(res.Annonces))
	}
	if res.Erreurs["A"] == nil {
		t.Error("l'erreur de la source A doit être rapportée")
	}
}

func TestChercherAnnoncesToutesEnPanne(t *testing.T) {
	avecSources(t, sourceFactice{nom: "A", err: errors.New("503")})
	if _, err := ChercherAnnonces(context.Background(), Criteres{}); err == nil {
		t.Error("toutes les sources en panne doit être une erreur")
	}
}

func TestChercherAnnoncesRepartitLesZones(t *testing.T) {
	// Un identifiant de zone n'a de sens que pour la source qui l'a émis :
	// l'envoyer à une autre produirait des résultats faux, sans erreur.
	var vuesA, vuesB []Criteres
	avecSources(t,
		sourceFactice{nom: "A", vues: &vuesA},
		sourceFactice{nom: "B", vues: &vuesB},
	)

	_, err := ChercherAnnonces(context.Background(), Criteres{Zones: []ZoneAnnonce{
		{Source: "A", ID: "za"},
		{Source: "B", ID: "zb1"},
		{Source: "B", ID: "zb2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(vuesA) != 1 || len(vuesA[0].Zones) != 1 || vuesA[0].Zones[0].ID != "za" {
		t.Errorf("la source A a reçu %+v", vuesA)
	}
	if len(vuesB) != 1 || len(vuesB[0].Zones) != 2 {
		t.Errorf("la source B a reçu %+v", vuesB)
	}
}

func TestChercherAnnoncesFiltreCeQueLaSourceNAPasFiltre(t *testing.T) {
	// Le cas mesuré sur Bien'ici : le portail accepte un filtre de terrain et
	// l'ignore. Le cœur doit rattraper.
	avecSources(t, sourceFactice{nom: "A", total: 2, annonces: []Annonce{
		{Source: "A", ID: "trop-petit", Type: BienTerrain, Prix: 50000, SurfaceTerrain: 300},
		{Source: "A", ID: "bon", Type: BienTerrain, Prix: 90000, SurfaceTerrain: 800},
	}})

	res, err := ChercherAnnonces(context.Background(), Criteres{TerrainMin: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Annonces) != 1 || res.Annonces[0].ID != "bon" {
		t.Errorf("le filtre local n'a pas rattrapé la source : %+v", res.Annonces)
	}
}

func TestChercherAnnoncesRespecteLaLimite(t *testing.T) {
	var as []Annonce
	for i := 0; i < 50; i++ {
		as = append(as, Annonce{
			Source: "A", ID: fmt.Sprintf("a%02d", i),
			Type: BienMaison, Prix: float64(i * 1000),
		})
	}
	avecSources(t, sourceFactice{nom: "A", total: 50, annonces: as})

	res, err := ChercherAnnonces(context.Background(), Criteres{Limite: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Annonces) != 5 {
		t.Errorf("%d annonces, attendu 5", len(res.Annonces))
	}
}

func TestCriteresResume(t *testing.T) {
	c := Criteres{
		Zones:      []ZoneAnnonce{{Nom: "Landes"}},
		Types:      []TypeBien{BienTerrain},
		SurfaceMax: 0, TerrainMax: 2000,
		PrixMax: 300000,
	}
	got := c.Resume()
	for _, attendu := range []string{"Landes", "Terrain nu", "300000", "2000"} {
		if !strings.Contains(got, attendu) {
			t.Errorf("résumé %q sans %q", got, attendu)
		}
	}
}
