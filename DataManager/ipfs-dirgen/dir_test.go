package ipfsdir

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDirgen(t *testing.T) {
	d := NewDir("Test")
	if err := d.AddLink("Subdir", "bafybeif5ruchubqhayrw2mezeinopghfj7gl3qlshq57bljirwokgm2dka", 100); err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(b))
}
