package logsym

import (
	"encoding/binary"
	"io"
	"os"
	"time"
	"unsafe"
)

// logfile is concerned with the internally rotated changing log data.
// The logfile has an uppersize limit which it will not write more than one
// log record over before starting at the beginning of the file.

// The LogFile object matching the xxxx.log on-disk file and used to hold the log recrods.
type LogFile struct {
	entryFile    *os.File // file pointer to the entry data
	metaFile     *os.File // meta file for header type info e.g. head pointer
	nextLogID    LSN      // current log sequence number that we are at
	headOffset   uint64   // offset into file where next record goes
	wrapNum      uint64   // how many times wrapped
	maxSizeBytes uint64   // maximum size in bytes that the log record can grow to
	byteOrder    binary.ByteOrder
}

// LSN is the log sequence number - the monoitically increasing next number for a log record
type LSN uint64

// The LogEntry entry or record to write to represent a log message
type LogEntry struct {
	logID     LSN    // privately set
	symbolID  SymID  // pointer to the symbol table information
	timeStamp uint64 // time when it is logged
	valueList []interface{}
}

type metaData struct {
	headOffset   uint64
	maxSizeBytes uint64
}

// CreateLogEntry creates a new log entry in memory
func CreateLogEntry(symbolID SymID, valueList []interface{}) (entry LogEntry) {
	entry.symbolID = symbolID
	entry.valueList = valueList
	entry.timeStamp = uint64(time.Now().UnixNano())
	return entry
}

// GetValues returns the associated values in a log entry
func (entry LogEntry) GetValues() []interface{} {
	return entry.valueList
}

// Go-type into log data ondisk-type
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
		case float32:
			t = TypeFloat32
		case float64:
			t = TypeFloat64
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
		case uint32, int32, float32:
			lenBytes = 4
		case uint64, int64, float64:
			lenBytes = 8
		case string:
			lenBytes = 4 // length prefix
			lenBytes += uint32(len(v))
		}
		totalLen += lenBytes
	}
	return totalLen
}

// The file name of the log data file
func logFileName(baseFileName string) string {
	return baseFileName + ".log"
}

func metaFileName(baseFileName string) string {
	return baseFileName + ".met"
}

// LogFileCreate creates a new log data file and allocates the LogFile data
func LogFileCreate(baseFileName string, maxFileSizeBytes int) (log *LogFile, err error) {
	entryFile, err := os.Create(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	metaFile, err := os.Create(metaFileName(baseFileName))
	if err != nil {
		entryFile.Close()
		return nil, err
	}

	log = &LogFile{
		byteOrder:    binary.LittleEndian,
		entryFile:    entryFile,
		metaFile:     metaFile,
		maxSizeBytes: uint64(maxFileSizeBytes),
	}
	return log, nil
}

// LogFileOpenRead opens a log file for reading and allocates the LogFile data
// It needs to position the log at the head of the tail of the log file.
// So subsequent reads can travers from the tail to the head.
func LogFileOpenRead(baseFileName string) (log *LogFile, err error) {
	// Open for reading from the start of the file
	f, err := os.Open(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}

	// We now need to find the tail of the log because that is our real start
	// start -> end
	// tail -> head
	//
	// head pointer in a header or work out by binary search for change in cycle#s.
	// Good to have a header anyway, but probably can just periodically update it
	// and confirm with a transition nearby - then don't need binary search
	// Also can update when log is finished being written to as we then want to be
	// quick from then onwards (more like forever in read-only mode).
	// Could have a file.sym file.log file.met
	//magic#
	//header-offset
}

// LogFileClose closes the associate files
func (log *LogFile) LogFileClose() error {
	err1 := log.entryFile.Close()
	err2 := log.metaFile.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

// Write metadata to the meta file
func (log *LogFile) writeMetaData(data metaData) error {
	// seek to start of the meta file
	_, err := log.metaFile.Seek(0, 0)
	if err != nil {
		return err
	}
	return binary.Write(log.metaFile, log.byteOrder, data)
}

// updateHead updates the meta file with the head pointer
func (log *LogFile) updateHead() error {
	// where are we in the log data file?
	offset, err := log.entryFile.Seek(0, 1)
	if err != nil {
		return err
	}

	log.headOffset = uint64(offset)

	data := metaData{headOffset: log.headOffset, maxSizeBytes: log.maxSizeBytes}
	return log.writeMetaData(data)
}

// LogFileAddEntry adds an entry to log data file
// This means writing to the file and possibly wrapping around and starting
// from the beginning of the file.
func (log *LogFile) LogFileAddEntry(entry LogEntry) (LSN, error) {

	// Wrap case
	// then we would go over the end of the log file
	// so start overwriting the beginning of the file
	if log.headOffset+uint64(entry.SizeBytes()) > log.maxSizeBytes {
		log.headOffset = 0
		log.wrapNum++
		log.entryFile.Seek(0, 0)
	}

	entry.logID = log.nextLogID

	// write our data to the file
	_, err = entry.Write(log.entryFile, log.byteOrder)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("len of write entry: %v\n", len)

	// update meta file - head points to next spot to write to
	// unless it is time to wrap to the start again
	err := log.updateHead()
	if err != nil {
		return 0, err
	}

	log.nextLogID++

	//fmt.Printf("log.nextLogID: %v\n", log.nextLogID)
	return log.nextLogID, nil
}

// SizeBytes returns # of bytes of the entry on disk
func (entry *LogEntry) SizeBytes() uint32 {
	var len uint32
	// sizes of fields on disk
	// 8 - 64 bit LSN
	// 8 - 64 bit sym Id
	// 8 - 64 bit timestamp
	// 4 - 32 bit length of value data
	// value-data
	headerLen := unsafe.Sizeof(entry.logID) +
		unsafe.Sizeof(entry.symbolID) +
		unsafe.Sizeof(entry.timeStamp) +
		unsafe.Sizeof(len)
	return uint32(headerLen) + sizeOfValues(entry.valueList)
}

// ReadEntry reads a log entry from the current position in the log file.
// Returns if there is an error, end of file or the entry.
func (log *LogFile) ReadEntry() (entry LogEntry, eof bool, err error) {
	// TODO
	return entry, eof, err
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
func (entry LogEntry) Write(w io.Writer, byteOrder binary.ByteOrder) (length int, err error) {

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
			// msgBytes := []byte(str)
			err = binary.Write(w, byteOrder, str)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
