package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestCarteRegions vérifie que les contours embarqués se projettent et que la
// carte occupe bien la place qu'on lui donne.
func TestCarteRegions(t *testing.T) {
	zones, err := Regions()
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 18 {
		t.Fatalf("18 régions attendues, %d obtenues", len(zones))
	}

	c := NouvelleCarte(zones, 90, 30, true, map[string]bool{"11": true})
	rendu := c.Rendu(8)

	lignes := strings.Split(rendu, "\n")
	if len(lignes) != 30 {
		t.Errorf("30 lignes attendues, %d obtenues", len(lignes))
	}
	fmt.Println(rendu)

	// Toute zone doit être atteignable : sinon elle est dans la liste mais pas
	// sur la carte, et le clic ne peut pas la trouver.
	for i, z := range zones {
		if c.surface[i] == 0 {
			t.Errorf("zone %s (%s) sans aucun pixel", z.Code, z.Nom)
		}
	}
}

// TestPointage vérifie la correspondance cellule -> zone, sur laquelle repose
// toute l'interaction à la souris.
func TestPointage(t *testing.T) {
	zones, err := DepartementsDe("75") // Nouvelle-Aquitaine
	if err != nil {
		t.Fatal(err)
	}
	c := NouvelleCarte(zones, 70, 26, false, nil)
	fmt.Println(c.Rendu(0))

	trouvees := map[int]bool{}
	for ligne := 0; ligne < c.H; ligne++ {
		for col := 0; col < c.L; col++ {
			if z := c.ZoneEn(col, ligne); z >= 0 {
				trouvees[z] = true
			}
		}
	}
	for i, z := range zones {
		if !trouvees[i] {
			t.Errorf("département %s (%s) non pointable", z.Code, z.Nom)
		}
	}
	if c.ZoneEn(-1, 0) != -1 || c.ZoneEn(0, c.H) != -1 {
		t.Error("un point hors carte doit renvoyer -1")
	}
}
