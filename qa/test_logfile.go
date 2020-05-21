package main

import (
	"fmt"
	"io"
	"strconv"
	"time"
    "os"
	"github.com/scorptec68/logsym"
)

// Test the log data file module

type CreateValFuncType func(int) (valueList []interface{}, keyTypes []logsym.KeyType)

func createValueKeyListFixed(i int) (valueList []interface{}, keyTypes []logsym.KeyType) {
	keyTypes = make([]logsym.KeyType, 6)
	keyTypes[0].Key = "bool key"
	keyTypes[0].ValueType = logsym.TypeBoolean
	keyTypes[1].Key = "u32 key"
	keyTypes[1].ValueType = logsym.TypeUint32
	keyTypes[2].Key = "u64 key"
	keyTypes[2].ValueType = logsym.TypeUint64
	keyTypes[3].Key = "f32 key"
	keyTypes[3].ValueType = logsym.TypeFloat32
	keyTypes[4].Key = "f64 key"
	keyTypes[4].ValueType = logsym.TypeFloat64
	keyTypes[5].Key = "string key"
	keyTypes[5].ValueType = logsym.TypeString

	// variables
	var b bool
	var x32 uint32
	var x64 uint64
	var f32 float32
	var f64 float64
	var s string

	b = true
	x32 = 42 + uint32(i)
	x64 = 43 + uint64(i)
	f32 = 3.1415 + float32(i)
	f64 = 0.623712 + float64(i)
	s = fmt.Sprintf("hi there %d", i)

	valueList = append(valueList, b)
	valueList = append(valueList, x32)
	valueList = append(valueList, x64)
	valueList = append(valueList, f32)
	valueList = append(valueList, f64)
	valueList = append(valueList, s)
	return valueList, keyTypes
}

func createValueKeyListVariable(i int) (valueList []interface{}, keyTypes []logsym.KeyType) {
	which := i % 6

	keyTypes = make([]logsym.KeyType, 1)
	
	switch which {
	case 0:
		keyTypes[0].Key = "bool key"
		keyTypes[0].ValueType = logsym.TypeBoolean
	case 1:
		keyTypes[0].Key = "u32 key"
		keyTypes[0].ValueType = logsym.TypeUint32
	case 2:
		keyTypes[0].Key = "u64 key"
		keyTypes[0].ValueType = logsym.TypeUint64
	case 3:
		keyTypes[0].Key = "f32 key"
		keyTypes[0].ValueType = logsym.TypeFloat32
	case 4:
		keyTypes[0].Key = "f64 key"
		keyTypes[0].ValueType = logsym.TypeFloat64
	case 5:
		keyTypes[0].Key = "string key"
		keyTypes[0].ValueType = logsym.TypeString
	}

	// variables
	var b bool
	var x32 uint32
	var x64 uint64
	var f32 float32
	var f64 float64
	var s string

	b = true
	x32 = 42 + uint32(i)
	x64 = 43 + uint64(i)
	f32 = 3.1415 + float32(i)
	f64 = 0.623712 + float64(i)
	s = fmt.Sprintf("hi there %d", i)

	switch which {
	case 0:
		valueList = append(valueList, b)
	case 1:
		valueList = append(valueList, x32)
	case 2:
		valueList = append(valueList, x64)
	case 3:
		valueList = append(valueList, f32)
	case 4:
		valueList = append(valueList, f64)
	case 5:
		valueList = append(valueList, s)
	}
	
	return valueList, keyTypes
}

func createAddEntries(log *logsym.LogFile, sym *logsym.SymFile, numEntries int, createValFunc CreateValFuncType) error {

	for i := 0; i < numEntries; i++ {
		valueList, keyTypes := createValFunc(i)
		iStr := strconv.Itoa(i)
		msgStr := "test message " + iStr
		fileStr := "testfile" + iStr
		lineNum := uint32(42 + i)

		symEntry := logsym.CreateSymbolEntry(msgStr, fileStr, lineNum, keyTypes)
		symID, err := sym.SymFileAddEntry(symEntry)
		if err != nil {
			return fmt.Errorf("Sym add error: %v", err)
		}

		//fmt.Printf("symid = %v\n", symID)

		logEntry := logsym.CreateLogEntry(symID, valueList)
		err = log.LogFileAddEntry(sym, logEntry)
		if err != nil {
			return fmt.Errorf("Log file add error: %v", err)
		}
	}
	return nil
}

func createLog(fname string, numEntries int, createValFunc CreateValFuncType, logSizeBytes uint64) (*logsym.LogFile, *logsym.SymFile, error) {

	log, err := logsym.LogFileCreate(fname, logSizeBytes)
	if err != nil {
		fmt.Printf("Log file create error: %v", err)
		return nil, nil, err
	}

	sym, err := logsym.SymFileCreate(fname)
	if err != nil {
		fmt.Printf("Sym create error: %v", err)
		log.LogFileClose()
		return nil, nil, err
	}

	err = createAddEntries(log, sym, numEntries, createValFunc)
	if err != nil {
		fmt.Printf("Log add entries error: %v", err)
		log.LogFileClose()
		sym.SymFileClose()
		return nil, nil, err
	}

	return log, sym, nil
}

func readPrintLogFile(fname string) {

	fmt.Printf("\n--- Read in log file ---\n")
	// open log for reading
	log, err := logsym.LogFileOpenRead(fname)
	if err != nil {
		fmt.Printf("Log open error: %v\n", err)
		return
	}
	defer log.LogFileClose()

	sym, err := logsym.SymFileReadAll(fname)
	if err != nil {
		fmt.Printf("Error reading in all of sym file: %v\n", err)
		return
	}
	defer sym.SymFileClose()

	var i uint64
	for i = 0; i < log.NumEntries; i++ {
		// Get the next entry
		pos, err := log.GetReadPos()
		if err != nil {
			fmt.Printf("Error reading position in log: %v", err)
		}
		fmt.Printf("Entry: %v, pos: %v\n", i, pos)
		readEntry, err := log.ReadEntry(sym)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Reached end of log file\n")
				break
			}
			fmt.Printf("Log read error: %v\n", err)
			break
		}

		// dump the data entry
		fmt.Printf("%v", readEntry.StringRelTime())

		// Get the associated symbol entry
		symEntry, ok := sym.SymFileGetEntry(readEntry.SymbolID())
		if ok {
			// dump the symbol entry
			fmt.Printf("%v\n", symEntry)
		}
	}
}

func ExampleDiffTimes() {
	fname := "testfile"
	log, sym, err := createLog(fname, 5, createValueKeyListFixed, 32*1024)
	if err != nil {
		fmt.Printf("Log create error: %v\n", err)
		return
	}
	
	time.Sleep(1 * time.Second)
	err = createAddEntries(log, sym, 3, createValueKeyListFixed)
	if err != nil {
		fmt.Printf("Add entries error: %v\n", err)
		return
	}
	
	time.Sleep(2 * time.Second)
	err = createAddEntries(log, sym, 1, createValueKeyListFixed)
	if err != nil {
		fmt.Printf("Add entries error: %v\n", err)
		return
	}
	
	log.LogFileClose()
	sym.SymFileWriteAll()
	sym.SymFileClose()
	fmt.Printf("%v", log)

	readPrintLogFile(fname)
}


func ExampleWrapPerfectFit() {
    fname := "testfile"
    
	//
	// Want entries to overflow log size for rotation to occur
	// So ensure that the log size is too small to handle all the entries
	// so that it will have to rotate and overwrite oldest entries.
	//
	numEntries := 4
	fmt.Printf("Number of log entries to write %v\n", numEntries)
	log, sym, err := createLog(fname, numEntries, createValueKeyListFixed, 280)
	if err != nil {
		fmt.Printf("Log create error: %v\n", err)
		return
	}
	log.LogFileClose()
	sym.SymFileWriteAll()
	sym.SymFileClose()
	
	fmt.Printf("\n--- The log file after creating it ---\n")
	fmt.Printf("%v", log)

    readPrintLogFile(fname)
}

func ExampleWrapVariableFit(numEntries int) {
	fname := "testfile"

	//
	// Want entries to overflow log size for rotation to occur
	// So ensure that the log size is too small to handle all the entries
	// so that it will have to rotate and overwrite oldest entries.
	//
	fmt.Printf("Number of log entries to write %v\n", numEntries)
	log, sym, err := createLog(fname, numEntries, createValueKeyListVariable, 200)
	if err != nil {
		fmt.Printf("Log create error: %v\n", err)
		return
	}
	err = log.LogFileClose()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	
	err = sym.SymFileWriteAll()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	
	err = sym.SymFileClose()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}

	fmt.Printf("\n--- The log file after creating it ---\n")
	fmt.Printf("%v", log)
	
	readPrintLogFile(fname)
}

func main() {

	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Printf("Input test number as an argument")
		return
	}

	cmd := args[0]
	numArg := 0
	var err error
	if len(args) > 1 {
		numArg, err = strconv.Atoi(args[1])
		if err != nil {
			return
		}
	}

	switch cmd {
	case "1": ExampleDiffTimes()
    case "2": ExampleWrapPerfectFit()
	case "3": ExampleWrapVariableFit(5)
	case "4": ExampleWrapVariableFit(numArg)
	}
}
