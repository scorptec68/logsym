package logsym

import (
	"fmt"
	"strconv"
)

// Test the symfile module

func ExampleSymFile() {

	sym, err := SymFileCreate("testfile")
	if err != nil {
		fmt.Printf("Sym create error: %v", err)
		return
	}

	keyTypes := make([]KeyType, 3)
	keyTypes[0].key = "jobid"
	keyTypes[0].valueType = TypeUint32
	keyTypes[1].key = "printerid"
	keyTypes[1].valueType = TypeString
	keyTypes[2].key = "lang"
	keyTypes[2].valueType = TypeString

	for i := 0; i < 5; i++ {
		iStr := strconv.Itoa(i)
		entry := CreateSymbolEntry("test message "+iStr, "testfile"+iStr, uint32(42+i), keyTypes)
		_, err := sym.SymFileAddEntry(entry)
		if err != nil {
			fmt.Printf("Sym add error: %v", err)
			return
		}
	}
	sym.SymFileClose()

	sym, err = SymFileOpenAppend("testfile")
	if err != nil {
		fmt.Printf("Sym open error: %v", err)
		return
	}
	fmt.Printf("Sym file: %v\n", sym)

	//Output:
}
