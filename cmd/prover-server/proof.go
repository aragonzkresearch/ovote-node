package main

import (
	"os/exec"
	"reflect"
	"time"

	"go.vocdoni.io/dvote/log"
)

const mutexLocked = 1

func (a *api) isBusy() bool {
	state := reflect.ValueOf(a).Elem().FieldByName("state")
	return state.Int()&mutexLocked == mutexLocked
}

// busyFor locks the mutex for s seconds. This method is for testing
// purposes only.
//
//nolint:unused
func (a *api) busyFor(s time.Duration) {
	a.Lock()
	time.Sleep(s)
	a.Unlock()
}

func genWitness(id string) error {
	// node ./circuit_js/generate_witness.js circuit.wasm zkinputs.json witness.wtns
	// TODO user cmd flag for path on generate_witness.js
	cmd := exec.Command("node", "./circuit_js/generate_witness.js", //nolint:gosec
		"circuit.wasm", "zkinputs"+id+".json", "witness"+id+".wtns")
	stdout, err := cmd.Output()

	if err != nil {
		log.Error("genWitness error:", err)
		return err
	}

	// Print the output
	log.Info("genWitness output:", string(stdout))
	return nil
}

func genProof(id string) error {
	// ~/bin/prover circuit.zkey witness.wtns proof.json public.json
	// TODO user cmd flag for path on prover
	cmd := exec.Command("./prover", "circuit.zkey", "witness"+id+".wtns", //nolint:gosec
		"proof"+id+".json", "public"+id+".json")
	stdout, err := cmd.Output()
	if err != nil {
		log.Error("genWitness error:", err)
		return err
	}

	// Print the output
	log.Info("proof output:", string(stdout))
	return nil
}

func (a *api) genWitnessAndProof(id string) {
	a.Lock()
	defer a.Unlock()

	if err := genWitness(id); err != nil {
		log.Error(err)
	}
	if err := genProof(id); err != nil {
		log.Error(err)
	}
}
