package logsym

import (
	"fmt"
	"testing"
)

// Test the log data file module

func createValueList(i int) (valueList []interface{}) {
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
	return valueList
}

func createAddEntries(log *LogFile, numEntries int) error {
	for i := 0; i < numEntries; i++ {
		valueList := createValueList(i)
		entry := CreateLogEntry(SymID(i), valueList)
		_, err := log.LogFileAddEntry(entry)
		if err != nil {
			return fmt.Errorf("Log file add error: %v", err)
		}
	}
	return nil
}

func createLog(fname string) (*LogFile, error) {
	logSize := 32 * 1024
	log, err := LogFileCreate(fname, logSize)
	if err != nil {
		fmt.Printf("Log file create error: %v", err)
		return nil, err
	}
	err = createAddEntries(log, 5)
	if err != nil {
		fmt.Printf("Log add entries error: %v", err)
		return nil, err
	}

	return log, nil
}

func TestLogFile(t *testing.T) {

	// prime up a log file with some entries in it
	log, err := createLog("testfile")
	if err != nil {
		t.Errorf("Failed to create log: %v", err)
		return
	}
	log.LogFileClose()

	// open log for reading
	log, err = LogFileOpenRead("testfile")
	if err != nil {
		t.Errorf("Log open error: %v", err)
		return
	}

	// read each entry and compare with what we expect should be there
	for i := 0; ; i++ {
		entry, eof, err := log.ReadEntry()
		if err != nil {
			t.Error(err)
			return
		}
		if eof {
			break
		}
		expectedValues := createValueList(i)
		for j, value := range entry.GetValues() {
			if value != expectedValues[j] {
				t.Errorf("Log data written and read in mismatch")
				t.Errorf("Wrote values: %v but read in %v", value, expectedValues[i])
				return

			}
		}
	}

	log.LogFileClose()
}

func ExampleLogFile() {

	log, err := createLog("testfile")
	if err != nil {
		fmt.Printf("Log create error: %v", err)
		return
	}
	err = createAddEntries(log, 3)
	if err != nil {
		fmt.Printf("Add entries error: %v", err)
		return
	}
	err = createAddEntries(log, 1)
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
