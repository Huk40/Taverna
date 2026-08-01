package assembly

import "testing"

func TestSocketTransformUsesBodyHeight(t *testing.T) {
	socket := socketTransform("head_mount", bodyHeightFor("heroic_body_01"), "idle_01")
	if socket.Position[1] != 2.8 {
		t.Fatalf("head_mount y = %v, want 2.8", socket.Position[1])
	}
}

func TestComposeWorldTransformAppliesCharacterScale(t *testing.T) {
	socket := Transform{
		Position: [3]float64{1, 2, 3},
		Rotation: [3]float64{10, 20, 30},
		Scale:    1,
	}
	user := Transform{
		Position: [3]float64{0.5, 0, -0.5},
		Rotation: [3]float64{5, -10, 15},
		Scale:    2,
	}

	world := composeWorldTransform(socket, user, 1.25)
	if world.Position != [3]float64{1.875, 2.5, 3.125} {
		t.Fatalf("world position = %v", world.Position)
	}
	if world.Rotation != [3]float64{15, 10, 45} {
		t.Fatalf("world rotation = %v", world.Rotation)
	}
	if world.Scale != 2.5 {
		t.Fatalf("world scale = %v", world.Scale)
	}
}
