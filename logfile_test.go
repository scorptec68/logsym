package logsym

import (
	"fmt"
	"strconv"
	"testing"
)

// Test the log data file module

func createValueKeyList(i int) (valueList []interface{}, keyTypes []KeyType) {
	keyTypes = make([]KeyType, 3)
	keyTypes[0].key = "bool key"
	keyTypes[0].valueType = TypeBoolean
	keyTypes[1].key = "u32 key"
	keyTypes[1].valueType = TypeUint32
	keyTypes[2].key = "u64 key"
	keyTypes[2].valueType = TypeUint64
	keyTypes[3].key = "f32 key"
	keyTypes[3].valueType = TypeFloat32
	keyTypes[4].key = "f64 key"
	keyTypes[4].valueType = TypeFloat64
	keyTypes[5].key = "string key"
	keyTypes[5].valueType = TypeString

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
	s = fmt.Sprint("hi there %d", i)

	valueList = append(valueList, b)
	valueList = append(valueList, x32)
	valueList = append(valueList, x64)
	valueList = append(valueList, f32)
	valueList = append(valueList, f64)
	valueList = append(valueList, s)
	return valueList, keyTypes
}

func createAddEntries(log *LogFile, sym *SymFile, numEntries int) error {

	for i := 0; i < numEntries; i++ {
		valueList, keyTypes := createValueKeyList(i)

		iStr := strconv.Itoa(i)
		symEntry := CreateSymbolEntry("test message "+iStr, "testfile"+iStr, uint32(42+i), keyTypes)
		symID, err := sym.SymFileAddEntry(symEntry)
		if err != nil {
			return fmt.Errorf("Sym add error: %v", err)
		}

		logEntry := CreateLogEntry(symID, valueList)
		err = log.LogFileAddEntry(logEntry)
		if err != nil {
			return fmt.Errorf("Log file add error: %v", err)
		}
	}
	return nil
}

func createLog(fname string, logSize int) (*LogFile, *SymFile, error) {

	log, err := LogFileCreate(fname, logSize)
	if err != nil {
		fmt.Printf("Log file create error: %v", err)
		return nil, nil, err
	}

	sym, err := SymFileCreate(fname)
	if err != nil {
		fmt.Printf("Sym create error: %v", err)
		log.LogFileClose()
		return nil, nil, err
	}

	err = createAddEntries(log, sym, 5)
	if err != nil {
		fmt.Printf("Log add entries error: %v", err)
		log.LogFileClose()
		sym.SymFileClose()
		return nil, nil, err
	}

	return log, sym, nil
}

func TestLogFile(t *testing.T) {
	// prime up a log file with some entries in it
	log, sym, err := createLog("testfile", 32*1024)
	if err != nil {
		t.Errorf("Failed to create log: %v", err)
		return
	}
	log.LogFileClose()
	sym.SymFileClose()

	// open log for reading
	log, err = LogFileOpenRead("testfile")
	if err != nil {
		t.Errorf("Log open error: %v", err)
		return
	}
	defer log.LogFileClose()

	sym, err = SymFileReadAll("testfile")
	if err != nil {
		t.Errorf("Failed to read in sym: %v", err)
		return
	}

	// read each entry and compare with what we expect should be there
	for i := 0; ; i++ {
		readEntry, eof, err := log.ReadEntry(sym)
		if err != nil {
			t.Error(err)
			return
		}
		if eof {
			break
		}
		expectedValues, expectedKeyTypes := createValueKeyList(i)
		for j, readValue := range readEntry.GetValues() {
			if readValue != expectedValues[j] {
				t.Errorf("Log data written and read in mismatch")
				t.Errorf("Wrote values: %v but read in %v", readValue, expectedValues[i])
				return

			}
		}
	}

}

func ExampleLogFile() {

	log, sym, err := createLog("testfile", 32*1024)
	if err != nil {
		fmt.Printf("Log create error: %v", err)
		return
	}
	err = createAddEntries(log, sym, 3)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	err = createAddEntries(log, sym, 1)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	log.LogFileClose()
	fmt.Printf("%v", log)

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
