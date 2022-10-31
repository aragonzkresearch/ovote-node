const path = require("path");
const fs = require("fs");
const { expect } = require("chai");

// to use c_tester need installed:
// apt install nlohmann-json3-dev nasm libgmp3-dev
// const c_tester = require("circom_tester").c;
const wasm_tester = require("circom_tester").wasm;
let circuitFile = "zkinputstest.circom";
let circuitPath = path.join(__dirname, "", circuitFile);

async function loadCircuit(nMaxVotes, nLevels) {
		console.log("generating circuit compiled files", nMaxVotes, nLevels);
		const circuitCode = `
			pragma circom 2.0.0;
			include "./node_modules/ovote/circuits/src/ovote.circom";
			component main {public [chainID, processID, censusRoot, receiptsRoot, nVotes, result, withReceipts]} = ovote(${nMaxVotes}, ${nLevels});
		`;
		await fs.writeFileSync(circuitPath, circuitCode, "utf8");

		console.log("going to load tester");
		// circuit = await c_tester(circuitPath);
		circuit = await wasm_tester(circuitPath);
		console.log("tester loaded, now going to circuit.loadConstraints");
		await circuit.loadConstraints();
		console.log("n_constraints", circuit.constraints.length);
	return circuit;
}

async function testCircuit(circuit, inputsFile) {
	const filePath = path.resolve(__dirname, inputsFile);
	const data = fs.readFileSync(filePath);
	const inputs = JSON.parse(data);

	console.log("calculateWitness");
	const witness = await circuit.calculateWitness(inputs, true);
	await circuit.checkConstraints(witness);
}

describe("TestZKInputs 3 nMaxVotes, 3 nLevels", function () {
	this.timeout(0);
	let circuit;

	const nMaxVotes = 3;
	const nLevels = 3;

	before( async() => {
		circuit = await loadCircuit(nMaxVotes, nLevels);
	});
	after( async() => {
		fs.unlinkSync(circuitPath);
	});

	const tests = [
		"zkinputs_3_3_1_60.json",
		"zkinputs_3_3_3_60.json",
	];

	for (let i=0; i<tests.length; i++) {
		it(tests[i], async () => {
			await testCircuit(circuit, tests[i]);
		});
	}
});

describe("TestZKInputs 8 nMaxVotes, 4 nLevels", function () {
	this.timeout(0);
	let circuitFile = "zkinputstest.circom";
	let circuitPath = path.join(__dirname, "", circuitFile);
	let circuit;

	const nMaxVotes = 8;
	const nLevels = 4;

	before( async() => {
		circuit = await loadCircuit(nMaxVotes, nLevels);
	});
	after( async() => {
		fs.unlinkSync(circuitPath);
	});

	const tests = [
		"zkinputs_8_4_3_60.json",
		"zkinputs_8_4_5_60.json",
		"zkinputs_8_4_7_60.json",
		"zkinputs_8_4_8_60.json",
	];

	for (let i=0; i<tests.length; i++) {
		it(tests[i], async () => {
			await testCircuit(circuit, tests[i]);
		});
	}
});
