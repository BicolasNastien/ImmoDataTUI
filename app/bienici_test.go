package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// echantillonBienici reproduit la forme exacte de la réponse du portail, y
// compris ses irrégularités : une annonce de programme neuf dont le prix, la
// surface et le nombre de pièces sont des *listes*, et une annonce sans
// surface habitable. Un échantillon lissé ne testerait que le code facile.
const echantillonBienici = `{
  "total": 7340,
  "realEstateAds": [
    {
      "id": "agence-123",
      "title": "Maison 5 pièces - Bayonne",
      "propertyType": "house",
      "price": 385000,
      "surfaceArea": 112.5,
      "landSurfaceArea": 620,
      "roomsQuantity": 5,
      "city": "Bayonne",
      "postalCode": "64100",
      "departmentCode": "64",
      "newProperty": false,
      "adCreatedByPro": true,
      "publicationDate": "2026-07-31T07:14:16.677Z",
      "priceWithoutFees": 367000,
      "feesChargedTo": "purchaser",
      "adType": "buy",
      "blurInfo": {"position": {"lat": 43.4933, "lon": -1.4748}}
    },
    {
      "id": "notaire-456",
      "title": "Terrain à bâtir - Castets",
      "propertyType": "terrain",
      "price": 89000,
      "surfaceArea": null,
      "landSurfaceArea": 812,
      "feesChargedTo": "seller",
      "priceWithoutFees": 0,
      "adType": "lifeAnnuitySale",
      "city": "Castets",
      "postalCode": "40260",
      "departmentCode": "40",
      "blurInfo": {"position": {"lat": 43.8783, "lon": -1.1497}}
    },
    {
      "id": "promoteur-789",
      "title": "Résidence Les Vignes",
      "propertyType": "programme",
      "price": [285000, 412000],
      "surfaceArea": [42.8, 61.6],
      "roomsQuantity": [2, 3],
      "city": "Frangy",
      "postalCode": "74270",
      "newProperty": true
    },
    {
      "id": "parking-999",
      "propertyType": "parking",
      "price": 7500,
      "city": "Bayonne"
    }
  ]
}`

func TestDecoderReponseBienici(t *testing.T) {
	var rep reponseBienici
	if err := json.Unmarshal([]byte(echantillonBienici), &rep); err != nil {
		t.Fatalf("une seule annonce de programme ne doit pas faire tomber la page : %v", err)
	}
	if rep.Total != 7340 || len(rep.Annonces) != 4 {
		t.Fatalf("total=%d, %d annonces", rep.Total, len(rep.Annonces))
	}

	maison := rep.Annonces[0].versAnnonce("Bien'ici")
	if maison.Type != BienMaison {
		t.Errorf("type = %v, attendu Maison", maison.Type)
	}
	if maison.Prix != 385000 || maison.Surface != 112.5 || maison.SurfaceTerrain != 620 {
		t.Errorf("valeurs mal décodées : %+v", maison)
	}
	if maison.Pieces != 5 || !maison.Pro || maison.Neuf {
		t.Errorf("drapeaux mal décodés : %+v", maison)
	}
	if maison.Lat == 0 || maison.Lon == 0 {
		t.Error("la position est dans blurInfo, pas à la racine")
	}
	if !strings.HasPrefix(maison.URL, urlFicheBienici) {
		t.Errorf("URL = %q", maison.URL)
	}
	attendu := time.Date(2026, 7, 31, 7, 14, 16, 0, time.UTC)
	if maison.Publiee.Sub(attendu) > time.Second {
		t.Errorf("date = %v", maison.Publiee)
	}

	// Honoraires à la charge de l'acheteur : l'acte portera le net vendeur.
	if maison.PrixCompare() != 367000 || maison.Viager {
		t.Errorf("prix de l'acte = %v, viager = %v", maison.PrixCompare(), maison.Viager)
	}

	terrain := rep.Annonces[1].versAnnonce("Bien'ici")
	// Honoraires à la charge du vendeur : l'acte porte le prix affiché.
	if terrain.PrixCompare() != 89000 || !terrain.Viager {
		t.Errorf("prix de l'acte = %v, viager = %v", terrain.PrixCompare(), terrain.Viager)
	}
	if terrain.Type != BienTerrain || terrain.Surface != 0 || terrain.SurfaceTerrain != 812 {
		t.Errorf("terrain mal décodé : %+v", terrain)
	}

	// Un programme neuf porte plusieurs lots : on retient le ticket d'entrée,
	// la seule valeur comparable à un bien unique.
	prog := rep.Annonces[2].versAnnonce("Bien'ici")
	if prog.Prix != 285000 || prog.Surface != 42.8 || prog.Pieces != 2 {
		t.Errorf("programme mal réduit : prix=%v surface=%v pièces=%d",
			prog.Prix, prog.Surface, prog.Pieces)
	}
	// « programme » n'est pas une nature de bien qu'on sait classer : il tombe
	// dans BienInconnu et sera écarté par le filtre de type.
	if prog.Type != BienInconnu {
		t.Errorf("type du programme = %v", prog.Type)
	}
	if Correspond(prog, Criteres{Types: TousTypes()}) {
		t.Error("un programme ne doit pas passer le filtre des trois types")
	}

	parking := rep.Annonces[3].versAnnonce("Bien'ici")
	if Correspond(parking, Criteres{Types: TousTypes()}) {
		t.Error("un parking ne doit pas passer le filtre des trois types")
	}
}

// Au-delà de deux mois, Bien'ici remplace la date de publication par le
// 1er janvier 1970 et donne un seuil dans thresholdDate.
func TestDecoderDatePubliceeMasquee(t *testing.T) {
	var a annonceBienici
	brut := `{"id": "x", "publicationDate": "1970-01-01T00:00:00.000Z",
		"modificationDate": "2026-10-26T08:45:50.405Z", "thresholdDate": "2026-08-28T00:00:00.000Z"}`
	if err := json.Unmarshal([]byte(brut), &a); err != nil {
		t.Fatal(err)
	}
	out := a.versAnnonce("Bien'ici")
	if !out.Publiee.IsZero() {
		t.Errorf("date 1970 retenue : %v", out.Publiee)
	}
	if want := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC); !out.PublieeAvant.Equal(want) {
		t.Errorf("seuil = %v, attendu %v", out.PublieeAvant, want)
	}

	// Une vraie date l'emporte, et la date de modification n'est jamais prise
	// pour celle de publication.
	brut = `{"id": "y", "publicationDate": "2026-10-23T16:44:05.977Z",
		"modificationDate": "2026-10-26T07:54:10.951Z"}`
	a = annonceBienici{}
	if err := json.Unmarshal([]byte(brut), &a); err != nil {
		t.Fatal(err)
	}
	out = a.versAnnonce("Bien'ici")
	if out.Publiee.Day() != 23 || !out.PublieeAvant.IsZero() {
		t.Errorf("publiée = %v, avant = %v", out.Publiee, out.PublieeAvant)
	}
}

func TestNombreBienici(t *testing.T) {
	cas := []struct {
		json string
		veut float64
	}{
		{`42`, 42},
		{`42.5`, 42.5},
		{`[100, 50, 75]`, 50}, // la plus petite : le ticket d'entrée
		{`null`, 0},
		{`[]`, 0},
		{`"350 000"`, 0}, // valeur inattendue : traitée comme absente
	}
	for _, c := range cas {
		var n nombreBienici
		if err := json.Unmarshal([]byte(c.json), &n); err != nil {
			t.Errorf("%s : %v", c.json, err)
			continue
		}
		if n.f() != c.veut {
			t.Errorf("%s → %v, attendu %v", c.json, n.f(), c.veut)
		}
	}
}

func TestConstruireFiltres(t *testing.T) {
	c := Criteres{
		Zones:      []ZoneAnnonce{{Source: "Bien'ici", ID: "-7376"}, {Source: "Bien'ici", ID: "-166713"}},
		Types:      []TypeBien{BienTerrain},
		SurfaceMax: 120,
		PrixMax:    300000,
		TerrainMin: 500, // le portail l'ignore : on ne doit pas le lui envoyer
		RayonKm:    10,  // idem
	}
	f := construireFiltres(c, 0, TriAnnonces{Critere: TriAPrix})

	if f.TypeFiltre != "buy" || len(f.EnVente) != 1 || !f.EnVente[0] {
		t.Error("la recherche doit porter sur les biens en vente et disponibles")
	}
	if f.Zones == nil || len(f.Zones.ZoneIDs) != 2 {
		t.Errorf("zones = %+v", f.Zones)
	}
	if f.TriPar != "price" || f.SensTri != "asc" {
		t.Errorf("tri = %s %s", f.TriPar, f.SensTri)
	}
	if f.SurfaceMax != 120 || f.PrixMax != 300000 {
		t.Errorf("bornes mal transmises : %+v", f)
	}

	// Les bornes non renseignées ne doivent pas partir du tout : envoyer
	// « minPrice: 0 » n'est pas la même chose que ne rien envoyer.
	brut, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, interdit := range []string{"minPrice", "minArea", "minRooms", "newProperty"} {
		if strings.Contains(string(brut), interdit) {
			t.Errorf("%s ne devait pas être envoyé : %s", interdit, brut)
		}
	}
	// Et les critères que le portail ignore ne doivent pas donner l'illusion
	// d'être appliqués côté serveur.
	for _, interdit := range []string{"LandSurface", "radius"} {
		if strings.Contains(string(brut), interdit) {
			t.Errorf("%s est ignoré par le portail, ne pas l'envoyer : %s", interdit, brut)
		}
	}

	// Les trois natures de bien couvrent bien toute la nomenclature du portail.
	tous := construireFiltres(Criteres{Types: TousTypes()}, 0, TriAnnonces{})
	if len(tous.TypesBien) < 10 {
		t.Errorf("%d types envoyés, la nomenclature du portail en compte plus", len(tous.TypesBien))
	}
}

func TestConstruireFiltresPagination(t *testing.T) {
	f := construireFiltres(Criteres{}, 2, TriAnnonces{})
	if f.Depuis != 2*pageBienici || f.Page != 3 || f.Taille != pageBienici {
		t.Errorf("pagination : from=%d page=%d size=%d", f.Depuis, f.Page, f.Taille)
	}
}

func TestConstruireFiltresExclureNeuf(t *testing.T) {
	f := construireFiltres(Criteres{ExclureNeuf: true}, 0, TriAnnonces{})
	brut, _ := json.Marshal(f)
	if !strings.Contains(string(brut), `"newProperty":false`) {
		t.Errorf("exclusion du neuf non transmise : %s", brut)
	}
}

func TestTriBienici(t *testing.T) {
	cas := []struct {
		tri  TriAnnonces
		par  string
		sens string
	}{
		{TriAnnonces{Critere: TriAPrix}, "price", "asc"},
		{TriAnnonces{Critere: TriAPrix, Decroissant: true}, "price", "desc"},
		{TriAnnonces{Critere: TriASurface}, "surface", "asc"},
		{TriAnnonces{Critere: TriADate, Decroissant: true}, "publicationDate", "desc"},
		// Le portail ne connaît ni le €/m² ni la surface de terrain : on lui
		// demande le prix et on retrie localement.
		{TriAnnonces{Critere: TriAPrixM2}, "price", "asc"},
		{TriAnnonces{Critere: TriATerrain}, "price", "asc"},
	}
	for _, c := range cas {
		par, sens := triBienici(c.tri)
		if par != c.par || sens != c.sens {
			t.Errorf("%v → %s %s, attendu %s %s", c.tri, par, sens, c.par, c.sens)
		}
	}
}

func TestEgalSansAccent(t *testing.T) {
	cas := []struct {
		a, b string
		veut bool
	}{
		{"Nouvelle-Aquitaine", "nouvelle aquitaine", true},
		{"Provence-Alpes-Côte d'Azur", "provence alpes cote d azur", true},
		{"Île-de-France", "ile de france", true},
		{"Landes", "Loire", false},
	}
	for _, c := range cas {
		if got := egalSansAccent(c.a, c.b); got != c.veut {
			t.Errorf("%q ≟ %q = %v, attendu %v", c.a, c.b, got, c.veut)
		}
	}
}

func TestEntierOuZero(t *testing.T) {
	cas := map[string]float64{
		"":        0,
		"250000":  250000,
		"250 000": 250000,
		"1,5":     1.5,
		"abc":     0, // un champ illisible vaut « pas de contrainte »
		"-5":      0,
	}
	for in, veut := range cas {
		if got := entierOuZero(in); got != veut {
			t.Errorf("%q → %v, attendu %v", in, got, veut)
		}
	}
}

func TestPremierNom(t *testing.T) {
	if got := premierNom("Bayonne/Bayona/Baiona"); got != "Bayonne" {
		t.Errorf("premierNom = %q", got)
	}
	if got := premierNom("Saint-Jean-de-Luz"); got != "Saint-Jean-de-Luz" {
		t.Errorf("premierNom = %q", got)
	}
}

// TestBieniciEnLigne interroge le vrai portail. Il est ignoré par `go test
// -short` : un test qui dépend du réseau n'a pas sa place dans la boucle de
// développement, mais l'écrire reste le seul moyen de savoir que l'adaptateur
// suit encore une API qui n'a rien promis.
func TestBieniciEnLigne(t *testing.T) {
	if testing.Short() {
		t.Skip("test réseau")
	}
	ctx, annuler := context.WithTimeout(context.Background(), 45*time.Second)
	defer annuler()

	var s sourceBienici
	zones, err := s.ResoudreZone(ctx, "Bayonne")
	if err != nil {
		t.Skipf("portail injoignable : %v", err)
	}
	if len(zones) == 0 {
		t.Fatal("aucune zone pour Bayonne")
	}

	as, total, err := s.Chercher(ctx, Criteres{
		Zones:  zones[:1],
		Types:  []TypeBien{BienMaison},
		Limite: 20,
		Tri:    TriAnnonces{Critere: TriAPrix},
	})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if len(as) == 0 || total == 0 {
		t.Fatalf("%d annonces sur %d annoncées", len(as), total)
	}
	for _, a := range as {
		if a.Prix <= 0 || a.Commune == "" || a.URL == "" {
			t.Errorf("annonce incomplète : %+v", a)
			break
		}
	}
}
