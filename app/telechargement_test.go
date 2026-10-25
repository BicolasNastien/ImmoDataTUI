package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// archiveFactice fabrique une archive départementale minimale dans le cache.
func archiveFactice(t *testing.T, annee, dep string, lignes ...string) {
	t.Helper()

	entete := "id_mutation,date_mutation,nature_mutation,valeur_fonciere,adresse_nom_voie," +
		"code_commune,nom_commune,type_local,surface_reelle_bati,nombre_pieces_principales," +
		"id_parcelle,code_nature_culture,surface_terrain"

	var brut bytes.Buffer
	brut.WriteString(entete + "\n")
	for _, l := range lignes {
		brut.WriteString(l + "\n")
	}

	var compresse bytes.Buffer
	gz := gzip.NewWriter(&compresse)
	if _, err := gz.Write(brut.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	chemin, err := CheminArchive(annee, dep)
	if err != nil {
		t.Fatal(err)
	}
	if err := ecrireAtomique(chemin, compresse.Bytes()); err != nil {
		t.Fatal(err)
	}
}

// TestExtractionCommune vérifie que le découpage d'une archive départementale
// produit une arborescence que le lecteur existant sait relire.
func TestExtractionCommune(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	archiveFactice(t, "2024", "64",
		"2024-1,2024-03-12,Vente,250000,RUE DES LILAS,64140,Boucau,Maison,90,4,641400000AB1,S,600",
		"2024-2,2024-04-02,Vente,180000,AV DU PORT,64102,Bayonne,Appartement,55,3,641020000AC9,,0",
	)

	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}
	if len(annees) != 1 || annees[0] != "2024" {
		t.Fatalf("millésimes attendus [2024], obtenus %v", annees)
	}

	cible := filepath.Join(racine, "Boucau", "2024", "64140.csv")
	if _, err := os.Stat(cible); err != nil {
		t.Fatalf("extrait absent : %v", err)
	}

	// Deux enregistrements : l'en-tête et la seule ligne de Boucau.
	if n, err := verifierCSV(cible); err != nil || n != 2 {
		t.Fatalf("extrait illisible (n=%d, err=%v)", n, err)
	}

	// Et surtout : le pipeline d'origine doit le consommer sans modification.
	d, err := ChargerDataset("Boucau", "2024", filepath.Dir(cible))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sales) != 1 || d.Sales[0].Type != "Maison" || d.Sales[0].Valeur != 250000 {
		t.Fatalf("consolidation inattendue : %+v", d.Sales)
	}
}

// TestExtractionCommuneVide ne doit rien laisser derrière elle.
func TestExtractionCommuneVide(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	archiveFactice(t, "2024", "64",
		"2024-2,2024-04-02,Vente,180000,AV DU PORT,64102,Bayonne,Appartement,55,3,641020000AC9,,0")

	annees, err := ExtraireCommune(racine, "64", "64140", "Boucau")
	if err != nil {
		t.Fatal(err)
	}
	if len(annees) != 0 {
		t.Fatalf("aucun millésime attendu, obtenu %v", annees)
	}
	if _, err := os.Stat(filepath.Join(racine, "Boucau", "2024")); !os.IsNotExist(err) {
		t.Error("un dossier vide a été laissé sur disque")
	}
}

func TestDetecterSeparateur(t *testing.T) {
	cas := []struct {
		entete string
		veut   rune
	}{
		{"id_mutation,date_mutation,valeur_fonciere", ','},
		{"id_mutation;date_mutation;valeur_fonciere", ';'},
	}
	for _, c := range cas {
		if got := detecterSeparateur(c.entete); got != c.veut {
			t.Errorf("%q : séparateur %q, attendu %q", c.entete, got, c.veut)
		}
	}
}

// TestCacheEtMarquage vérifie le repérage de ce qui est déjà téléchargé, qui
// sert à teinter la carte.
func TestCacheEtMarquage(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	archiveFactice(t, "2024", "64")

	if !ArchiveFraiche("2024", "64") {
		t.Error("l'archive qu'on vient d'écrire devrait être fraîche")
	}
	if ArchiveFraiche("2023", "64") {
		t.Error("un millésime absent ne peut pas être frais")
	}
	if deps := DepartementsTelecharges(); !deps["64"] {
		t.Errorf("département 64 attendu dans %v", deps)
	}
	if regs := RegionsAvecDonnees(map[string]bool{"64": true}); !regs["75"] {
		t.Errorf("région 75 (Nouvelle-Aquitaine) attendue dans %v", regs)
	}
}

// TestTelechargementReel touche le réseau : ignoré en mode court.
func TestTelechargementReel(t *testing.T) {
	if testing.Short() {
		t.Skip("réseau")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	r := TelechargerArchive(context.Background(), "2024", "48", false)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.Absent || r.Octets < 1000 {
		t.Fatalf("archive suspecte : %+v", r)
	}

	racine := t.TempDir()
	annees, err := ExtraireCommune(racine, "48", "48092", "Marvejols")
	if err != nil {
		t.Fatal(err)
	}
	if len(annees) == 0 {
		t.Fatal("aucun millésime extrait pour Marvejols")
	}

	d, err := ChargerDataset("Marvejols", annees[0], filepath.Join(racine, "Marvejols", annees[0]))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Marvejols %s : %d mutations, %d ventes exploitables", annees[0], d.Mutations, len(d.Sales))
	if len(d.Sales) == 0 {
		t.Error("aucune vente consolidée")
	}
}

// TestPublicationSuitLExtrait vérifie que la date de publication notée à côté
// de l'archive arrive jusqu'au jeu de données d'une commune, avec la date de la
// dernière vente.
func TestPublicationSuitLExtrait(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	racine := t.TempDir()

	archiveFactice(t, "2024", "64",
		"2024-1,2024-03-12,Vente,250000,RUE DES LILAS,64140,Boucau,Maison,90,4,641400000AB1,S,600",
		"2024-2,2024-09-30,Vente,210000,RUE DES ROSES,64140,Boucau,Maison,80,4,641400000AB2,S,500",
	)
	archive, _ := CheminArchive("2024", "64")
	publie := time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)
	if err := EcrirePublication(archive+suffixePublication, publie); err != nil {
		t.Fatal(err)
	}

	if _, err := ExtraireCommune(racine, "64", "64140", "Boucau"); err != nil {
		t.Fatal(err)
	}
	d, err := ChargerDataset("Boucau", "2024", filepath.Join(racine, "Boucau", "2024"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Publication.Equal(publie) {
		t.Errorf("publication attendue %v, obtenue %v", publie, d.Publication)
	}
	if got := d.DerniereVente.Format("2006-01-02"); got != "2024-09-30" {
		t.Errorf("dernière vente attendue 2024-09-30, obtenue %s", got)
	}
	if d.Fichiers != 1 {
		t.Errorf("la note de publication ne doit pas compter comme fichier : %d", d.Fichiers)
	}
}

// TestFusionDesDates : la fusion garde la vente la plus récente, mais la
// publication la plus ancienne — et l'inconnue l'emporte sur tout.
func TestFusionDesDates(t *testing.T) {
	jour := func(a, m, j int) time.Time { return time.Date(a, time.Month(m), j, 0, 0, 0, 0, time.UTC) }

	f := Fusionner([]Dataset{
		{Annee: "2023", Publication: jour(2025, 4, 1), DerniereVente: jour(2023, 12, 28)},
		{Annee: "2024", Publication: jour(2025, 10, 1), DerniereVente: jour(2024, 12, 30)},
	})
	if !f.Publication.Equal(jour(2025, 4, 1)) || !f.DerniereVente.Equal(jour(2024, 12, 30)) {
		t.Errorf("dates fusionnées inattendues : %v / %v", f.Publication, f.DerniereVente)
	}

	f = Fusionner([]Dataset{
		{Annee: "2023", Publication: jour(2025, 4, 1)},
		{Annee: "2024"},
	})
	if !f.Publication.IsZero() {
		t.Errorf("une publication inconnue doit rendre la fusion inconnue : %v", f.Publication)
	}
}
