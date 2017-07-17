package tmx

import (
	"os"
	"path"
	"testing"
)

func TestDecoder(t *testing.T) {
	file, err := os.Open(path.Join("fixtures", "test.tmx"))
	if err != nil {
		t.Fatalf(err.Error())
	}

	m, err := Decode(file)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if m == nil {
		t.Fatalf("map was nil")
	}

	if walls := m.LayerWithName("walls"); walls != nil {
		trs, err := walls.TileGlobalRefs()
		if err != nil {
			t.Error(err)
		}

		if l, e := len(trs), m.Width*m.Height; l != e {
			t.Errorf("expected tiles of length %v, got %v", e, l)
		}

		tds, err := walls.TileDefs(m.TileSets)
		if err != nil {
			t.Error(err)
		}

		// we have three known tiles and IDs as our first few tiles; check those
		type Expected struct {
			tsName              string
			tID                 TileID
			HorizontallyFlipped bool
			VerticallyFlipped   bool
			DiagonallyFlipped   bool
		}
		exp := []Expected{
			Expected{"temp", 142, false, false, false},
			Expected{"temp", 141, false, false, false},
			Expected{"temp", 141, false, false, false},
			Expected{"temp", 127, false, false, false},
			Expected{"temp", 127, true, false, true},
			Expected{"temp", 127, true, true, false},
			Expected{"temp", 127, false, true, true},
			Expected{"temp", 127, false, false, false},
		}

		for i, e := range exp {
			tile := tds[i]
			if tile.TileSet.Name != e.tsName {
				t.Errorf("idx(%v): expected tileset named `%v`, got `%v`", i, e.tsName, tile.TileSet.Name)
			}
			if tile.ID != e.tID {
				t.Errorf("idx(%v): expected tile id `%v`, got `%v`", i, e.tID, tile.ID)
			}
			if tile.HorizontallyFlipped != e.HorizontallyFlipped {
				t.Errorf("idx(%v): expected tile to be horizontally flipped: %v", i, e.HorizontallyFlipped)
			}
			if tile.VerticallyFlipped != e.VerticallyFlipped {
				t.Errorf("idx(%v): expected tile to be vertically flipped: %v", i, e.VerticallyFlipped)
			}
			if tile.DiagonallyFlipped != e.DiagonallyFlipped {
				t.Errorf("idx(%v): expected tile to be diagonally flipped: %v", i, e.DiagonallyFlipped)
			}

		}
	} else {
		t.Error("expected layer with name `walls`, but found none.")
	}

	if nonSolid := m.LayerWithName("non-solid"); nonSolid != nil {
		trs, err := nonSolid.TileGlobalRefs()
		if err != nil {
			t.Error(err)
		}

		if l, e := len(trs), m.Width*m.Height; l != e {
			t.Errorf("expected tiles of length %v, got %v", e, l)
		}
	} else {
		t.Error("expected layer with name `non-solid`, but found none.")
	}

	if players := m.ObjectGroupWithName("players"); players != nil {
	} else {
		t.Error("expected objectgroup with name `players`, but found none.")
	}

	if ts := m.TileSetWithName("temp"); ts != nil {
	} else {
		t.Error("expected tileset with name `temp`, but found none.")
	}

	if enemies := m.ObjectGroupWithName("enemies"); enemies != nil {
		if enemy := enemies.Objects.WithName("enemy1"); enemy != nil {
			if cool, err := enemy.Properties.Bool("cool"); err != nil {
				t.Errorf("unexpected error getting property `cool`")
			} else if cool != true {
				t.Errorf("expected property `cool` to be true")
			}

			if velX, err := enemy.Properties.Float("velX"); err != nil {
				t.Errorf("unexpected error getting property `velX`")
			} else if velX != 1.1 {
				t.Errorf("expected property `velX` to have value `1.1`")
			}

			if health, err := enemy.Properties.Int("health"); err != nil {
				t.Errorf("unexpected error getting property `health`")
			} else if health != 100 {
				t.Errorf("expected property `health` to have value `100`")
			}

			if food := enemy.Properties.WithName("food"); food == nil {
				t.Error("expected to find property named food, found none.")
			} else if food.Value != "pizza" {
				t.Error("expected property `food` to have value `pizza`")
			}
		} else {
			t.Errorf("expected object with name `enemy1`, but found none.")
		}
	} else {
		t.Error("expected objectgroup with name `enemies`, but found none.")
	}
}
