package logsym

import (
	"os"
	"time"
)

// logfile is concerned with the internally rotated changing log data.
// The logfile has an uppersize limit which it will not write more than one
// log record over before starting at the beginning of the file.

// The LogFile object matching the xxxx.log on-disk file and used to hold the log recrods.
type LogFile struct {
	file         *os.File
	nextLogID    LSN    // current log sequence number that we are at
	headOffset   uint64 // offset into file where next record goes
	wrapNum      uint64 // how many times wrapped
	maxSizeBytes uint64 // maximum size in bytes that the log record can grow to
}

// LSN is the log sequence number - the monoitically increasing next number for a log record
type LSN uint64

// The LogEntry entry or record to write to represent a log message
type LogEntry struct {
	logID     LSN
	symbolID  SymID
	timeStamp uint64
	valueList []interface{}
	// Hmmm... valueList can be of the type as used in the api - native go types
	// or as the data types used on disk - log types
}

// SizeBytes returns # of bytes of the entry on disk
func (entry *LogEntry) SizeBytes() uint32 {
	return 8 + 8 + 8 + 4 + sizeOfValues(entry.valueList)
}

func sizeOfValues(valueList []interface{}) uint32 {
	// map native go types to log types

}

/*
 * [160 bit SHA-1 hash - too big]
 * 64 bit LSN
 * 64 bit sym Id
 * 64 bit timestamp
 * 32 bit length of value data
 * ...value data...
 * <types stored in the sym file>
 * type sizes: 8 bit, 32 bit, 64 bit, 32 bit len + len bytes
 */
func wow() {
	t := time.Now()
	t.UnixNano()
}

// The file name of the log data file
func logFileName(baseFileName string) string {
	return baseFileName + ".log"
}

// LogFileCreate creates a new log data file and allocates the LogFile data
func LogFileCreate(baseFileName string, maxFileSizeBytes uint64) (log *LogFile, err error) {
	f, err := os.Create(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	log = &LogFile{
		file:         f,
		maxSizeBytes: maxFileSizeBytes,
	}
	return log, nil
}

// LogFileAddEntry adds an entry to log data file
func (log *LogFile) LogFileAddEntry(entry LogEntry) (LSN, error) {
	entry.logID = log.nextLogID
	if log.headOffset+uint64(entry.SizeBytes()) > log.maxSizeBytes {
		// then we would go over the end of the log file
		// so start overwriting the beginning of the file
		log.headOffset = 0
	}
	len, err := entry.Write(log.file, log.headOffset)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("len of write entry: %v\n", len)

	log.nextLogID++

	//fmt.Printf("log.nextLogID: %v\n", log.nextLogID)
	return log.nextLogID, nil
}
