package logsym

import (
	"encoding/binary"
	"io"
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

func nativeToTypeList(valueList []interface{}) []LogValueType {
	t := TypeByteData
	typeList := make([]LogValueType, len(valueList))
	for _, val := range valueList {
		switch val.(type) {
		case uint8:
			t = TypeUint8
		case int8:
			t = TypeInt8
		case uint32:
			t = TypeUint32
		case int32:
			t = TypeInt32
		case uint64:
			t = TypeUint64
		case int64:
			t = TypeInt64
		case bool:
			t = TypeBoolean
		case string:
			t = TypeString
		}
		typeList = append(typeList, t)
	}
	return typeList
}

// Calculate number of bytes for all the different values in the list
// This size is for ondisk
func sizeOfValues(valueList []interface{}) uint32 {
	var totalLen, lenBytes uint32
	for _, val := range valueList {
		switch v := val.(type) {
		case uint8, int8, bool:
			lenBytes = 1
		case uint32, int32:
			lenBytes = 4
		case uint64, int64:
			lenBytes = 8
		case string:
			lenBytes = 4 // length prefix
			lenBytes += uint32(len(v))
		}
		totalLen += lenBytes
	}
	return totalLen
}

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
		log.wrapNum++
		log.file.Seek(0, 0)
	}
	_, err := entry.Write(log.file)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("len of write entry: %v\n", len)

	log.nextLogID++

	//fmt.Printf("log.nextLogID: %v\n", log.nextLogID)
	return log.nextLogID, nil
}

// SizeBytes returns # of bytes of the entry on disk
func (entry *LogEntry) SizeBytes() uint32 {
	// sizes of fields on disk
	// 64 bit LSN
	// 64 bit sym Id
	// 64 bit timestamp
	// 32 bit length of value data
	// value-data
	return 8 + 8 + 8 + 4 + sizeOfValues(entry.valueList)
}

/*
 * 64 bit LSN
 * 64 bit sym Id
 * 64 bit timestamp
 * 32 bit length of value data
 * ...value data...
 * <types stored in the sym file>
 * type sizes: 8 bit, 32 bit, 64 bit, 32 bit len + len bytes
 */
func (entry LogEntry) Write(w io.Writer) (length int, err error) {
	byteOrder := binary.LittleEndian

	// logID LSN
	length += binary.Size(entry.logID)
	err = binary.Write(w, byteOrder, entry.logID)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("logId: %v\n", entry.logID)

	// symID
	length += binary.Size(entry.symbolID)
	err = binary.Write(w, byteOrder, entry.symbolID)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("symId: %v\n", entry.symbolID)

	// timestamp
	length += binary.Size(entry.timeStamp)
	err = binary.Write(w, byteOrder, entry.timeStamp)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("timestamp: %v\n", entry.timeStamp)

	// length of value data
	length += 4
	valLen := sizeOfValues(entry.valueList)
	err = binary.Write(w, byteOrder, valLen)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("value len: %v\n", valLen)

	// value data
	// Note: no type info is stored in the data log
	// Type info is kept in the sym file
	length += int(valLen)
	err = writeValueList(w, byteOrder, entry.valueList)
	if err != nil {
		return 0, err
	}

	return length, nil
}

func writeValueList(w io.Writer, byteOrder binary.ByteOrder, valueList []interface{}) (err error) {
	for _, value := range valueList {

		switch value.(type) {
		case uint8, int8, bool, uint32, int32, uint64, int64:
			// fixed size
			err = binary.Write(w, byteOrder, value)
			if err != nil {
				return err
			}
		case string:
			// length prefix
			var l uint32
			str := value.(string)
			l = uint32(len(str))
			err = binary.Write(w, byteOrder, l)
			if err != nil {
				return err
			}

			// string data
			// ? str to byte array?
			// msgBytes := []byte(entry.message)
			err = binary.Write(w, byteOrder, l)
			if err != nil {
				return err
			}
		}
	}

	return nil
}