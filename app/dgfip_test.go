package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const enteteBrut = "Identifiant de document|Reference document|1 Articles CGI|2 Articles CGI|" +
	"3 Articles CGI|4 Articles CGI|5 Articles CGI|No disposition|Date mutation|Nature mutation|" +
	"Valeur fonciere|No voie|B/T/Q|Type de voie|Code voie|Voie|Code postal|Commune|" +
	"Code departement|Code commune|Prefixe de section|Section|No plan|No Volume|1er lot|" +
	"Surface Carrez du 1er lot|2eme lot|Surface Carrez du 2eme lot|3eme lot|" +
	"Surface Carrez du 3eme lot|4eme lot|Surface Carrez du 4eme lot|5eme lot|" +
	"Surface Carrez du 5eme lot|Nombre de lots|Code type local|Type local|Identifiant local|" +
	"Surface reelle bati|Nombre pieces principales|Nature culture|Nature culture speciale|" +
	"Surface terrain"

// ligneBrute place des valeurs par nom de colonne : compter quarante-trois
// barres verticales à la main est le plus sûr moyen de se tromper.
func ligneBrute(valeurs map[string]string) string {
	noms := strings.Split(enteteBrut, "|")
	champs := make([]string, len(noms))
	for i, nom := range noms {
		champs[i] = valeurs[nom]
	}
	return strings.Join(champs, "|")
}

// venteBrute : les champs communs à toutes les lignes de test.
func venteBrute(date, valeur, dep, commune string, autres map[string]string) string {
	v := map[string]string{
		"No disposition": "000001", "Date mutation": date, "Nature mutation": "Vente",
		"Valeur fonciere": valeur, "Code departement": dep, "Code commune": commune,
	}
	for k, x := range autres {
		v[k] = x
	}
	return ligneBrute(v)
}

// zipFactice écrit un fichier national minimal, tel que la DGFiP le publie.
func zipFactice(t *testing.T, lignes ...string) string {
	t.Helper()
	chemin := filepath.Join(t.TempDir(), "valeursfoncieres-2026-s1.txt.zip")
	f, err := os.Create(chemin)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("ValeursFoncieres-2026-S1.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(enteteBrut + "\r\n" + strings.Join(lignes, "\r\n") + "\r\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return chemin
}

func TestCodeInsee(t *testing.T) {
	cas := []struct{ dep, commune, attendu string }{
		{"01", "244", "01244"},
		{"64", "24", "64024"},
		{"2A", "4", "2A004"},
		{"971", "10", "97110"},
		{"971", "", ""},
	}
	for _, c := range cas {
		if got := CodeInsee(c.dep, c.commune); got != c.attendu {
			t.Errorf("CodeInsee(%q, %q) = %q, attendu %q", c.dep, c.commune, got, c.attendu)
		}
	}
}

func TestIDParcelle(t *testing.T) {
	if got := IDParcelle("01244", "", "G", "135"); got != "012440000G0135" {
		t.Errorf("parcelle %q", got)
	}
	if got := IDParcelle("97117", "", "BA", "195"); got != "97117000BA0195" {
		t.Errorf("parcelle outre-mer %q", got)
	}
	if got := IDParcelle("64024", "", "", "12"); got != "" {
		t.Errorf("sans section, pas de parcelle : %q", got)
	}
}

func TestChoisirSourcesDGFiP(t *testing.T) {
	d := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	r := func(nom string) ressourceCatalogue {
		return ressourceCatalogue{URL: "https://static.data.gouv.fr/x/" + nom, LastModified: d}
	}
	catalogue := []ressourceCatalogue{
		r("valeursfoncieres-2026-s2.txt.zip"),
		r("valeursfoncieres-2026-s1.txt.zip"),
		r("valeursfoncieres-2025.txt.zip"),
		r("faq-20221017.pdf"),
	}

	s := ChoisirSourcesDGFiP(catalogue, "2026")
	if len(s) != 2 || s[0].Nom != "valeursfoncieres-2026-s1.txt.zip" {
		t.Fatalf("semestres attendus dans l'ordre, obtenus %+v", s)
	}

	// L'année complète rend ses semestres inutiles.
	catalogue = append(catalogue, r("valeursfoncieres-2026.txt.zip"))
	if s := ChoisirSourcesDGFiP(catalogue, "2026"); len(s) != 1 || s[0].Nom != "valeursfoncieres-2026.txt.zip" {
		t.Fatalf("année complète attendue, obtenu %+v", s)
	}

	if s := ChoisirSourcesDGFiP(catalogue, "2019"); len(s) != 0 {
		t.Fatalf("aucun fichier attendu, obtenu %+v", s)
	}
}

// TestConversionDGFiP suit un fichier brut jusqu'à l'écran de statistiques :
// l'archive convertie doit passer par le découpage communal et la
// consolidation sans que ni l'un ni l'autre ne sache d'où elle vient.
func TestConversionDGFiP(t *testing.T) {
	isolerCache(t)
	racine := t.TempDir()

	adresse := map[string]string{
		"No voie": "12", "Type de voie": "RUE", "Code voie": "0410", "Voie": "DES LILAS",
		"Code postal": "64340", "Commune": "BOUCAU",
	}
	avec := func(base map[string]string, ajouts map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range ajouts {
			out[k] = v
		}
		return out
	}

	z := zipFactice(t,
		// Une maison sur deux parcelles : deux lignes, une seule mutation.
		venteBrute("08/01/2026", "250000,00", "64", "140", avec(adresse, map[string]string{
			"Section": "AB", "No plan": "12", "Code type local": "1", "Type local": "Maison",
			"Surface reelle bati": "92", "Nombre pieces principales": "4",
			"Nature culture": "S", "Surface terrain": "600"})),
		venteBrute("08/01/2026", "250000,00", "64", "140", avec(adresse, map[string]string{
			"Section": "AB", "No plan": "13", "Nature culture": "T",
			"Nature culture speciale": "POTAG", "Surface terrain": "300"})),
		// Un appartement, mutation suivante.
		venteBrute("09/01/2026", "180000,00", "64", "140", avec(adresse, map[string]string{
			"Section": "AC", "No plan": "9", "1er lot": "7", "Surface Carrez du 1er lot": "54,30",
			"Nombre de lots": "1", "Code type local": "2", "Type local": "Appartement",
			"Surface reelle bati": "55", "Nombre pieces principales": "3"})),
		// Un autre département : écarté.
		venteBrute("08/01/2026", "130000,00", "01", "244", map[string]string{
			"Section": "G", "No plan": "135", "Type local": "Appartement", "Surface reelle bati": "41"}),
	)

	archive, err := CheminArchive("2026", "64")
	if err != nil {
		t.Fatal(err)
	}
	lignes, _, err := convertirDGFiP([]string{z}, "2026", "64", archive)
	if err != nil {
		t.Fatal(err)
	}
	if lignes != 3 {
		t.Fatalf("3 lignes attendues pour le 64, obtenu %d", lignes)
	}
	if err := ecrireAtomique(archive+suffixeProvisoire, []byte("DGFiP\n")); err != nil {
		t.Fatal(err)
	}

	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}
	if len(annees) != 1 || annees[0] != "2026" {
		t.Fatalf("millésime 2026 attendu, obtenu %v", annees)
	}

	d, err := ChargerDataset("Boucau", "2026", filepath.Join(racine, "Boucau", "2026"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Provisoire {
		t.Error("la marque provisoire n'a pas suivi l'extrait")
	}
	if len(d.Sales) != 2 {
		t.Fatalf("2 ventes attendues, obtenu %d : %+v", len(d.Sales), d.Sales)
	}

	ventes := map[string]Sale{}
	for _, s := range d.Sales {
		ventes[s.Type] = s
	}
	m := ventes["Maison"]
	if m.Valeur != 250000 || m.SurfaceBati != 92 || m.Date.Format("2006-01-02") != "2026-01-08" {
		t.Errorf("maison mal convertie : %+v", m)
	}
	if m.SurfaceTerrain != 900 {
		t.Errorf("terrain cumulé attendu 900 m², obtenu %v", m.SurfaceTerrain)
	}
	if m.CodePostal != "64340" || m.Voie != "RUE DES LILAS" {
		t.Errorf("adresse mal convertie : %q %q", m.CodePostal, m.Voie)
	}
	if p := m.Lignes[0].IDParcelle; p != "64140000AB0012" && p != "64140000AB0013" {
		t.Errorf("parcelle mal reconstituée : %q", p)
	}
	if _, _, ok := m.Position(); ok {
		t.Error("le fichier brut n'a pas de coordonnées")
	}
	if a := ventes["Appartement"]; a.Valeur != 180000 {
		t.Errorf("appartement mal converti : %+v", a)
	}
}

// TestConversionDGFiPAbsent : un département absent du fichier brut
// (l'Alsace-Moselle n'y figure pas) ne doit laisser aucune archive.
func TestConversionDGFiPAbsent(t *testing.T) {
	isolerCache(t)
	z := zipFactice(t, venteBrute("08/01/2026", "130000,00", "01", "244", nil))

	archive, err := CheminArchive("2026", "67")
	if err != nil {
		t.Fatal(err)
	}
	lignes, _, err := convertirDGFiP([]string{z}, "2026", "67", archive)
	if err != nil || lignes != 0 {
		t.Fatalf("aucune ligne attendue (n=%d, err=%v)", lignes, err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Error("une archive vide a été laissée dans le cache")
	}
}
