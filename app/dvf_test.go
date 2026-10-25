package main

import (
	"fmt"
	"testing"
)

// racineDonnees est la collection déposée à la main, à la racine du dépôt.
const racineDonnees = "../data"

// premierDataset renvoie le premier couple ville/année de la collection locale,
// ou ignore le test si elle est vide : les données ne sont pas versionnées.
func premierDataset(t *testing.T) Dataset {
	t.Helper()
	villes, err := ScannerRacine(racineDonnees)
	if err != nil || len(villes) == 0 {
		t.Skipf("aucune collection locale dans %s", racineDonnees)
	}
	v := villes[0]
	d, err := ChargerDataset(v.Nom, v.Annees[0].Nom, v.Annees[0].Chemin)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestConsolidation(t *testing.T) {
	d := premierDataset(t)
	fmt.Printf("Fichiers=%d Mutations=%d Ventes=%d Ecartees=%d\n",
		d.Fichiers, d.Mutations, len(d.Sales), d.Ecartees)

	// Maisons 90-150 m² : doit retrouver les ventes vues en analyse Python.
	var n int
	for _, s := range d.Sales {
		if s.Type == "Maison" && s.SurfaceBati >= 90 && s.SurfaceBati <= 150 {
			n++
			fmt.Printf("  %10.0f € %5.0f m² terrain %6.0f m² %s\n",
				s.Valeur, s.SurfaceBati, s.SurfaceTerrain, s.Voie)
		}
	}
	fmt.Println("Maisons 90-150 m² retenues:", n)

	st := StatsPourType(d.Sales, "Maison")
	fmt.Printf("MAISONS n=%d med=%.0f q1=%.0f q3=%.0f m2med=%.0f\n",
		st.Nb, st.Prix.Med, st.Prix.Q1, st.Prix.Q3, st.PrixM2.Med)

	sa := StatsPourType(d.Sales, "Appartement")
	fmt.Printf("APPARTS n=%d med=%.0f m2med=%.0f\n", sa.Nb, sa.Prix.Med, sa.PrixM2.Med)

	pt, pm2 := StatsTerrain(d.Sales)
	fmt.Printf("TERRAINS n=%d med=%.0f m2med=%.0f (q1=%.0f q3=%.0f)\n",
		pt.Nb, pt.Med, pm2.Med, pm2.Q1, pm2.Q3)

	for _, tr := range ParTrancheSurface(d.Sales, "Maison") {
		fmt.Printf("  %-14s %3d  %10.0f  %8.0f\n", tr.Label, tr.Nb, tr.PrixMed, tr.PrixM2Med)
	}

	if len(d.Sales) == 0 {
		t.Error("aucune vente consolidée, parsing probablement cassé")
	}
	_ = n
}

func TestToutesLesVilles(t *testing.T) {
	villes, err := ScannerRacine(racineDonnees)
	if err != nil {
		t.Fatal(err)
	}
	if len(villes) == 0 {
		t.Skipf("aucune collection locale dans %s", racineDonnees)
	}
	for _, v := range villes {
		for _, a := range v.Annees {
			d, err := ChargerDataset(v.Nom, a.Nom, a.Chemin)
			if err != nil {
				t.Errorf("%s/%s: %v", v.Nom, a.Nom, err)
				continue
			}
			st := StatsPourType(d.Sales, "Maison")
			fmt.Printf("%-28s %s  ventes=%3d ecart=%3d  maisons=%3d med=%9.0f €  %5.0f €/m²\n",
				v.Nom, a.Nom, len(d.Sales), d.Ecartees, st.Nb, st.Prix.Med, st.PrixM2.Med)
		}
	}
}

func TestFormatage(t *testing.T) {
	cas := []struct {
		in   float64
		want string
	}{
		{485000, "485 000 €"},
		{1200000, "1 200 000 €"},
		{8500, "8 500 €"},
		{950, "950 €"},
		{0, "–"},
	}
	for _, c := range cas {
		if got := euros(c.in); got != c.want {
			t.Errorf("euros(%v) = %q, attendu %q", c.in, got, c.want)
		}
	}
}
