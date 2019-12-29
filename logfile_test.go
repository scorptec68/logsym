package logsym

import (
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"
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

		//fmt.Printf("symid = %v\n", symID)

		logEntry := CreateLogEntry(symID, valueList)
		err = log.LogFileAddEntry(sym, logEntry)
		if err != nil {
			return fmt.Errorf("Log file add error: %v", err)
		}
	}
	return nil
}

func createLog(fname string, numEntries int, logSize uint64) (*LogFile, *SymFile, error) {

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

	err = createAddEntries(log, sym, numEntries)
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
	log, sym, err := createLog("testfile", 5, 32*1024)
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

	// 1. Setting up the log and writing to the disk file

	log, sym, err := createLog("testfile", 5, 32*1024)
	if err != nil {
		fmt.Printf("Log create error: %v", err)
		return
	}
	time.Sleep(1 * time.Second)
	err = createAddEntries(log, sym, 3)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	time.Sleep(2 * time.Second)
	err = createAddEntries(log, sym, 1)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	log.LogFileClose()
	fmt.Printf("%v", log)

	// 2. Reading back the log file

	// open log for reading
	log, err = LogFileOpenRead("testfile")
	if err != nil {
		fmt.Printf("Log open error: %v", err)
		return
	}
	defer log.LogFileClose()

	for i := 0; ; i++ {
		readEntry, err := log.ReadEntry(sym)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Reached end of log file\n")
				break
			}
			fmt.Printf("Log read error: %v", err)
			return
		}

		fmt.Printf("%v", readEntry.StringRel())
		symEntry, ok := sym.SymFileGetEntry(readEntry.SymbolID())
		if ok {
			fmt.Printf("%v\n", symEntry)
		}
	}
	// Output:
	// LogFile
	//   nextLogId: {0 9}
	//   headOffset: 675
	//   tailOffset: 0
	//   wrapNum: 0
	//   maxSizeBytes: 32768
	//   byteOrder: LittleEndian
	// LogEntry
	//   logId: {0 0}
	//   symId: 0
	//   timeStamp rel: now - 3 secs
	//   value: true
	//   value: 42
	//   value: 43
	//   value: 3.1415
	//   value: 0.623712
	//   value: hi there 0
	// Symbol Entry <symId: 0, numAccesses: 2, level: DEBUG, message: "test message 0", fname: "testfile0", line: 42, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 1}
	//   symId: 132
	//   timeStamp rel: now - 3 secs
	//   value: true
	//   value: 43
	//   value: 44
	//   value: 4.1415
	//   value: 1.623712
	//   value: hi there 1
	// Symbol Entry <symId: 132, numAccesses: 1, level: DEBUG, message: "test message 1", fname: "testfile1", line: 43, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 2}
	//   symId: 264
	//   timeStamp rel: now - 3 secs
	//   value: true
	//   value: 44
	//   value: 45
	//   value: 5.1415
	//   value: 2.6237120000000003
	//   value: hi there 2
	// Symbol Entry <symId: 264, numAccesses: 1, level: DEBUG, message: "test message 2", fname: "testfile2", line: 44, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 3}
	//   symId: 396
	//   timeStamp rel: now - 3 secs
	//   value: true
	//   value: 45
	//   value: 46
	//   value: 6.1415
	//   value: 3.6237120000000003
	//   value: hi there 3
	// Symbol Entry <symId: 396, numAccesses: 0, level: DEBUG, message: "test message 3", fname: "testfile3", line: 45, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 4}
	//   symId: 528
	//   timeStamp rel: now - 3 secs
	//   value: true
	//   value: 46
	//   value: 47
	//   value: 7.1415
	//   value: 4.623712
	//   value: hi there 4
	// Symbol Entry <symId: 528, numAccesses: 0, level: DEBUG, message: "test message 4", fname: "testfile4", line: 46, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 5}
	//   symId: 0
	//   timeStamp rel: now - 2 secs
	//   value: true
	//   value: 42
	//   value: 43
	//   value: 3.1415
	//   value: 0.623712
	//   value: hi there 0
	// Symbol Entry <symId: 0, numAccesses: 2, level: DEBUG, message: "test message 0", fname: "testfile0", line: 42, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 6}
	//   symId: 132
	//   timeStamp rel: now - 2 secs
	//   value: true
	//   value: 43
	//   value: 44
	//   value: 4.1415
	//   value: 1.623712
	//   value: hi there 1
	// Symbol Entry <symId: 132, numAccesses: 1, level: DEBUG, message: "test message 1", fname: "testfile1", line: 43, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 7}
	//   symId: 264
	//   timeStamp rel: now - 2 secs
	//   value: true
	//   value: 44
	//   value: 45
	//   value: 5.1415
	//   value: 2.6237120000000003
	//   value: hi there 2
	// Symbol Entry <symId: 264, numAccesses: 1, level: DEBUG, message: "test message 2", fname: "testfile2", line: 44, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// LogEntry
	//   logId: {0 8}
	//   symId: 0
	//   timeStamp rel: now - 0 secs
	//   value: true
	//   value: 42
	//   value: 43
	//   value: 3.1415
	//   value: 0.623712
	//   value: hi there 0
	// Symbol Entry <symId: 0, numAccesses: 2, level: DEBUG, message: "test message 0", fname: "testfile0", line: 42, keyTypes: [<key: "bool key", type: Boolean> <key: "u32 key", type: Uint32> <key: "u64 key", type: Uint64> <key: "f32 key", type: Float32> <key: "f64 key", type: Float64> <key: "string key", type: String>]>
	// Reached end of log file
}

func ExampleLogFile2() {

	// 1. Setting up the log and writing to the disk file

	log, sym, err := createLog("testfile", 5, 300)
	if err != nil {
		fmt.Printf("Log create error: %v", err)
		return
	}

	log.LogFileClose()
	fmt.Printf("%v", log)

	// open log for reading
	log, err = LogFileOpenRead("testfile")
	if err != nil {
		fmt.Printf("Log open error: %v", err)
		return
	}
	defer log.LogFileClose()

	for i := 0; ; i++ {
		readEntry, err := log.ReadEntry(sym)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Reached end of log file\n")
				break
			}
			fmt.Printf("Log read error: %v", err)
			return
		}

		fmt.Printf("%v", readEntry.StringRel())
		symEntry, ok := sym.SymFileGetEntry(readEntry.SymbolID())
		if ok {
			fmt.Printf("%v\n", symEntry)
		}
	}

	// Output:
	// LogFile
}
