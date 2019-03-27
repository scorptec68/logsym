package logsym

import (
	"fmt"
	"strconv"
	"testing"
)

// Test the symfile module

func createAddSymEntries(sym *SymFile, numEntries int) error {
	keyTypes := make([]KeyType, 3)
	keyTypes[0].key = "jobid"
	keyTypes[0].valueType = TypeUint32
	keyTypes[1].key = "printerid"
	keyTypes[1].valueType = TypeString
	keyTypes[2].key = "lang"
	keyTypes[2].valueType = TypeString

	for i := 0; i < numEntries; i++ {
		iStr := strconv.Itoa(i)
		entry := CreateSymbolEntry("test message "+iStr, "testfile"+iStr, uint32(42+i), keyTypes)
		_, err := sym.SymFileAddEntry(entry)
		if err != nil {
			return fmt.Errorf("Sym add error: %v", err)
		}
	}
	return nil
}

func createSym(fname string) (*SymFile, error) {
	sym, err := SymFileCreate(fname)
	if err != nil {
		fmt.Printf("Sym create error: %v", err)
		return nil, err
	}
	err = createAddSymEntries(sym, 5)
	if err != nil {
		fmt.Printf("Sym add entries error: %v", err)
		return nil, err
	}

	return sym, nil
}

func TestSymFile(t *testing.T) {
	sym1, err := createSym("testfile")
	if err != nil {
		t.Errorf("Failed to create sym: %v", err)
		return
	}
	sym1.SymFileClose()

	sym2, err := SymFileOpenAppend("testfile")
	if err != nil {
		t.Errorf("Sym open error: %v", err)
		return
	}

	if sym1.String() != sym2.String() {
		t.Errorf("Symbol written and read in mismatch")
		t.Errorf("Wrote: %v but read in %v", sym1, sym2)
		return
	}

	sym2.SymFileClose()
}

func ExampleSymFile() {

	sym, err := createSym("testfile")
	if err != nil {
		fmt.Printf("Sym create error: %v", err)
		return
	}
	err = createAddSymEntries(sym, 3)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	err = createAddSymEntries(sym, 1)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	sym.SymFileClose()
	fmt.Printf("%v", sym)

	// Output:
	// SymFile
	//   nextSymId: 445
	//   entries:
	//     Key: "test message 0testfile042"
	//     Value: Entry<symId: 0, numAccesses: 2, level: DEBUG, message: "test message 0", fname: "testfile0", line: 42, keyTypes: [<key: "jobid", type: Uint32> <key: "printerid", type: String> <key: "lang", type: String>]>
	//     Key: "test message 1testfile143"
	//     Value: Entry<symId: 89, numAccesses: 1, level: DEBUG, message: "test message 1", fname: "testfile1", line: 43, keyTypes: [<key: "jobid", type: Uint32> <key: "printerid", type: String> <key: "lang", type: String>]>
	//     Key: "test message 2testfile244"
	//     Value: Entry<symId: 178, numAccesses: 1, level: DEBUG, message: "test message 2", fname: "testfile2", line: 44, keyTypes: [<key: "jobid", type: Uint32> <key: "printerid", type: String> <key: "lang", type: String>]>
	//     Key: "test message 3testfile345"
	//     Value: Entry<symId: 267, numAccesses: 0, level: DEBUG, message: "test message 3", fname: "testfile3", line: 45, keyTypes: [<key: "jobid", type: Uint32> <key: "printerid", type: String> <key: "lang", type: String>]>
	//     Key: "test message 4testfile446"
	//     Value: Entry<symId: 356, numAccesses: 0, level: DEBUG, message: "test message 4", fname: "testfile4", line: 46, keyTypes: [<key: "jobid", type: Uint32> <key: "printerid", type: String> <key: "lang", type: String>]>
}
