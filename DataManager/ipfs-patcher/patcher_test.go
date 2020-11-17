package patcher

import (
	"fmt"
	"testing"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	populatedRoot = "bafybeia7lfhpwujl5ykq5nbgmx2r6nkjmm6xntuimd6bivdv3jstc2wdle"
	testDirRoot   = "bafybeihzp34gcdwvnfvs7kvayyciic4b4yslcmft52g44hkujj4x577rai"
	gameOfLife    = "bafkreihfswq76qcoh57hhqs53tsnjvl37o4ydmm6zelgj67ml5pcdvfxm4"
	manyPatched   = "bafybeicnnegjprdrwij5gw6xxaawmbucju4s3xnfi4xii4qm3lvlchymie"
)

func TestPatch(t *testing.T) {
	patcher := NewPatcher(shell.NewLocalShell(), "")
	err := patcher.Cp("test/game_of_life.png", gameOfLife)
	if err != nil {
		t.Fatal(err)
	}
	if patcher.Root() != populatedRoot {
		t.Fatalf("root is: %s Expecting %s", patcher.Root(), populatedRoot)
	}
}

func TestAddMany(t *testing.T) {
	patcher := NewPatcher(shell.NewLocalShell(), "")

	var finished int

	patch := func(i int) {
		err := patcher.Cp(
			fmt.Sprintf(
				"%d/%d.game_of_life.png",
				i%10,
				i,
			),
			gameOfLife,
		)
		if err != nil {
			t.Fatal(err)
		}

		finished++
	}

	for i := 0; i < 100; i++ {
		go patch(i)
	}

	for finished < 100 {
	}

	if patcher.Root() != manyPatched {
		t.Fatalf("root is %s Expecting %s", patcher.Root(), manyPatched)
	}
}

func TestRm(t *testing.T) {
	patcher := NewPatcher(shell.NewLocalShell(), populatedRoot)

	err := patcher.Rm("test/game_of_life.png")
	if err != nil {
		t.Fatal(err)
	}

	if patcher.Root() != testDirRoot {
		t.Fatalf("root is: %s Expecting %s", patcher.Root(), testDirRoot)
	}

	err = patcher.Rm("test/game_of_life.png")
	if err == nil {
		t.Fatal("Error is nil when it shouldn't be")
	}

	err = patcher.Rm("test")
	if err != nil {
		t.Fatal(err)
	}

	if patcher.Root() != cidV1UnixfsDir {
		t.Fatalf("root is: %s Expecting %s", patcher.Root(), cidV1UnixfsDir)
	}
}
