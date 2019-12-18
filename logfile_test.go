package logsym

import (
	"fmt"
	"io"
	"strconv"
	"testing"
)

// Test the log data file module

func createValueKeyList(i int) (valueList []interface{}, keyTypes []KeyType) {
	keyTypes = make([]KeyType, 6)
	keyTypes[0].Key = "bool key"
	keyTypes[0].ValueType = TypeBoolean
	keyTypes[1].Key = "u32 key"
	keyTypes[1].ValueType = TypeUint32
	keyTypes[2].Key = "u64 key"
	keyTypes[2].ValueType = TypeUint64
	keyTypes[3].Key = "f32 key"
	keyTypes[3].ValueType = TypeFloat32
	keyTypes[4].Key = "f64 key"
	keyTypes[4].ValueType = TypeFloat64
	keyTypes[5].Key = "string key"
	keyTypes[5].ValueType = TypeString

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

func createAddEntries(log *LogFile, sym *SymFile, numEntries int) error {

	for i := 0; i < numEntries; i++ {
		valueList, keyTypes := createValueKeyList(i)

		iStr := strconv.Itoa(i)
		msgStr := "test message " + iStr
		fileStr := "testfile" + iStr
		lineNum := uint32(42 + i)
		symEntry := CreateSymbolEntry(msgStr, fileStr, lineNum, keyTypes)
		symID, err := sym.SymFileAddEntry(symEntry)
		if err != nil {
			return fmt.Errorf("Sym add error: %v", err)
		}

		logEntry := CreateLogEntry(symID, valueList)
		err = log.LogFileAddEntry(sym, logEntry)
		if err != nil {
			return fmt.Errorf("Log file add error: %v", err)
		}
	}
	return nil
}

func createLog(fname string, logSize uint64) (*LogFile, *SymFile, error) {

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
		readEntry, err := log.ReadEntry(sym)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Error(err)
			return
		}
		expectedValues, _ := createValueKeyList(i)
		for j, readValue := range readEntry.GetValues() {
			if readValue != expectedValues[j] {
				t.Errorf("Log data written and read in mismatch")
				t.Errorf("Wrote values: %v but read in %v", expectedValues[i], readValue)
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
	// LogFile
	//   nextSymId: 445
}
