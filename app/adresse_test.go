package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// enteteDVF : les colonnes utilisées par les fixtures de ce fichier.
const enteteDVF = "id_mutation,date_mutation,nature_mutation,valeur_fonciere," +
	"adresse_numero,adresse_nom_voie,code_postal,code_commune,nom_commune," +
	"type_local,surface_reelle_bati,nombre_pieces_principales," +
	"id_parcelle,code_nature_culture,surface_terrain,longitude,latitude"

// millesimeFactice écrit un extrait de commune là où le programme l'attend.
func millesimeFactice(t *testing.T, racine, commune, annee, insee string, lignes ...string) {
	t.Helper()
	dir := filepath.Join(racine, NormaliserNom(commune), annee)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contenu := enteteDVF + "\n" + strings.Join(lignes, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, insee+".csv"), []byte(contenu), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParserAdresses(t *testing.T) {
	brut := []byte(`{"type":"FeatureCollection","features":[
	 {"type":"Feature","geometry":{"type":"Point","coordinates":[4.0889,48.3040]},
	  "properties":{"label":"12 Rue des Lilas 64140 Boucau","housenumber":"12",
	   "street":"Rue des Lilas","postcode":"64140","city":"Boucau","citycode":"64140",
	   "context":"64, Pyrénées-Atlantiques, Nouvelle-Aquitaine","type":"housenumber"}},
	 {"type":"Feature","geometry":{"type":"Point","coordinates":[2.35,48.85]},
	  "properties":{"label":"Boucau","name":"Boucau","postcode":"64140","city":"Boucau",
	   "citycode":"64140","type":"municipality"}},
	 {"type":"Feature","geometry":{"type":"Point","coordinates":[0,0]},
	  "properties":{"label":"sans code commune","type":"street"}}]}`)

	res, err := parserAdresses(brut)
	if err != nil {
		t.Fatal(err)
	}
	// Le troisième résultat n'a pas de code commune : inexploitable, écarté.
	if len(res) != 2 {
		t.Fatalf("2 adresses exploitables attendues, obtenues %d", len(res))
	}
	a := res[0]
	if a.Numero != "12" || a.Voie != "Rue des Lilas" || a.CodeInsee != "64140" {
		t.Errorf("adresse mal décodée : %+v", a)
	}
	if !a.EstPrecise() {
		t.Error("un housenumber devrait être une adresse précise")
	}
	if a.Departement() != "64" {
		t.Errorf("département 64 attendu, obtenu %s", a.Departement())
	}
	if a.Lat != 48.3040 || a.Lon != 4.0889 {
		t.Errorf("coordonnées mal lues : %v %v", a.Lat, a.Lon)
	}
	if res[1].EstPrecise() {
		t.Error("une commune n'est pas une adresse précise")
	}

	// Outre-mer : trois chiffres. Corse : les deux premiers suffisent.
	if d := (Adresse{CodeInsee: "97105"}).Departement(); d != "971" {
		t.Errorf("département 971 attendu, obtenu %s", d)
	}
	if d := (Adresse{CodeInsee: "2A004"}).Departement(); d != "2A" {
		t.Errorf("département 2A attendu, obtenu %s", d)
	}
}

func TestRapprochementVoie(t *testing.T) {
	memes := [][2]string{
		{"AV DU PORT", "Avenue du Port"},
		{"RUE DES LILAS", "rue des lilas"},
		{"BD DU 14 JUILLET", "Boulevard du 14 Juillet"},
		{"CHE DE LA CROIX", "Chemin de la Croix"},
		{"RUE GENERAL LECLERC", "Rue du Général Leclerc"},
		{"IMP DES ROSES", "Impasse des Roses"},
	}
	for _, c := range memes {
		if !memeVoie(c[0], c[1]) {
			t.Errorf("%q et %q devraient se rapprocher (%q / %q)",
				c[0], c[1], cleVoie(c[0]), cleVoie(c[1]))
		}
	}
	differentes := [][2]string{
		{"RUE DES LILAS", "Rue des Roses"},
		{"AV DU PORT", "Rue du Port"},
		{"", "Rue des Lilas"},
	}
	for _, c := range differentes {
		if memeVoie(c[0], c[1]) {
			t.Errorf("%q et %q ne devraient pas se rapprocher", c[0], c[1])
		}
	}

	if !memeNumero("12", "12 BIS") || memeNumero("12", "14") || memeNumero("", "12") {
		t.Error("comparaison de numéros incorrecte")
	}
}

// TestPageAdresse vérifie l'arrivée sur la page d'une adresse : millésimes
// agrégés, onglet Adresse en tête, curseur posé sur une vente du bon numéro.
func TestPageAdresse(t *testing.T) {
	racine := t.TempDir()
	millesimeFactice(t, racine, "Boucau", "2023", "64140",
		"2023-1,2023-04-11,Vente,240000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,-1.47,43.52",
		"2023-2,2023-06-02,Vente,150000,30,AV DU PORT,64140,64140,Boucau,Appartement,60,3,641400000AB7,,0,-1.47,43.52",
	)
	millesimeFactice(t, racine, "Boucau", "2024", "64140",
		"2024-1,2024-03-12,Vente,265000,12,AV DU PORT,64140,64140,Boucau,Maison,95,4,641400000AB1,S,500,-1.47,43.52",
		"2024-2,2024-05-02,Vente,310000,4,RUE DES LILAS,64140,64140,Boucau,Maison,120,5,641400000AB2,S,800,-1.47,43.52",
	)

	adresse := Adresse{
		Label: "12 Avenue du Port 64140 Boucau", Numero: "12", Voie: "Avenue du Port",
		CodePostal: "64140", Commune: "Boucau", CodeInsee: "64140",
		Type: "housenumber", Lat: 43.52, Lon: -1.47,
	}

	mm := NouveauModele(racine, nil)
	mm.adresse = &adresse
	mm.commune = Zone{Code: "64140", Nom: "Boucau"}

	var m tea.Model = mm
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 120, Height: 40},
		msgExtraction{commune: mm.commune, annees: []string{"2024", "2023"}},
	)

	got := m.(model)
	if got.ecran != ecranStats {
		t.Fatal("une recherche d'adresse devrait mener directement aux statistiques")
	}
	if got.onglets[0] != "Adresse" || got.curOnglet != 0 {
		t.Fatalf("onglet Adresse attendu en tête, obtenu %v", got.onglets)
	}
	if len(got.parAnnee) != 2 || len(got.total.Sales) != 4 {
		t.Fatalf("2 millésimes et 4 ventes agrégées attendus, obtenus %d et %d",
			len(got.parAnnee), len(got.total.Sales))
	}
	if got.total.Annee != "2023 – 2024" {
		t.Errorf("période 2023 – 2024 attendue, obtenue %q", got.total.Annee)
	}

	// Le tableau ne retient que l'avenue du Port, pas la rue des Lilas.
	ventes := got.ventesOnglet()
	if len(ventes) != 3 {
		t.Fatalf("3 ventes dans la voie attendues, obtenues %d", len(ventes))
	}
	// Le curseur se pose sur une vente au numéro 12.
	if v := ventes[got.curVente]; v.Numero != "12" {
		t.Errorf("curseur attendu sur le numéro 12, obtenu %q", v.Numero)
	}

	fmt.Println("### PAGE ADRESSE ###")
	fmt.Println(m.View())

	vue := sansCouleur(m.View())
	for _, attendu := range []string{
		"12 Avenue du Port", "À CETTE ADRESSE", "LA VOIE — AVENUE DU PORT",
		"LA COMMUNE", "265 000 €", "240 000 €",
	} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("la page devrait contenir %q", attendu)
		}
	}
	if strings.Contains(vue, "RUE DES LILAS") {
		t.Error("la page ne devrait pas lister une autre voie")
	}

	// La fiche s'ouvre depuis la page d'adresse comme depuis un autre onglet.
	m = envoyer(m, touche("enter"))
	if m.(model).ecran != ecranVente {
		t.Error("↵ devrait ouvrir la fiche de la vente sélectionnée")
	}
}

// TestEcranRecherche exerce la saisie sans réseau : la commande de recherche
// est produite mais son résultat est injecté à la main.
func TestEcranRecherche(t *testing.T) {
	regions, err := Regions()
	if err != nil {
		t.Fatal(err)
	}
	var m tea.Model = NouveauModele(t.TempDir(), nil)
	m = envoyer(m,
		tea.WindowSizeMsg{Width: 110, Height: 40},
		msgZones{niveau: nivRegions, zones: regions},
		touche("/"),
	)
	if m.(model).ecran != ecranRecherche {
		t.Fatal("/ devrait ouvrir la recherche d'adresse")
	}

	// « q » se tape dans un nom de rue au lieu de quitter le programme.
	m = envoyer(m, touche("q"), touche("u"), touche("a"), touche("i"))
	if v := m.(model).saisie.Value(); v != "quai" {
		t.Fatalf("saisie « quai » attendue, obtenue %q", v)
	}

	m = envoyer(m, msgAdresses{requete: "quai", resultats: []Adresse{
		{Label: "Quai de Dampierre 10000 Troyes", Voie: "Quai de Dampierre",
			Commune: "Troyes", CodeInsee: "10387", Type: "street"},
		{Label: "12 Quai des Comtes 10000 Troyes", Numero: "12", Voie: "Quai des Comtes",
			Commune: "Troyes", CodeInsee: "10387", Type: "housenumber"},
	}})
	got := m.(model)
	if len(got.resultats) != 2 || got.curResultat != 0 {
		t.Fatalf("2 résultats attendus, obtenus %d", len(got.resultats))
	}
	fmt.Println("### RECHERCHE ###")
	fmt.Println(m.View())

	m = envoyer(m, touche("down"))
	if m.(model).curResultat != 1 {
		t.Error("↓ devrait descendre dans les résultats")
	}
	m = envoyer(m, touche("esc"))
	if m.(model).ecran != ecranCarte {
		t.Error("esc devrait revenir à la carte")
	}
}
