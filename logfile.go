package logsym

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"
)

// logfile is concerned with the internally rotated changing log data.
// The logfile has an uppersize limit which it will not write more than one
// log record over before starting at the beginning of the file.
//
// Assumptions:
// Writing and reading will not be interleaved.
// The writing will occur first.
// The reading will start on a file which has finished writing.

// The LogFile object matching the xxxx.log on-disk file and used to hold the log recrods.
type LogFile struct {
	entryFile    *os.File         // file pointer to the entry data
	metaFile     *os.File         // meta file for header type info e.g. head pointer
	nextLogID    LSN              // current log sequence number that we are at
	headOffset   uint64           // offset into file where next record goes
	tailOffset   uint64	          // offset into file where earliest record is
	wrapNum      uint64           // how many times wrapped
	maxSizeBytes uint64           // maximum size in bytes that the log record can grow to
	byteOrder    binary.ByteOrder // the byte ordering of the numbers
}

// LSN is the log sequence number - the monotonically increasing next number for a log record
type LSN struct {
	Cycle uint64 // number of cycles or wraps of log
	Pos   uint64 // position within the log file - nth entry (not an offset)
}

// The LogEntry entry or record to write to represent a log message
type LogEntry struct {
	logID     LSN           // privately set
	symbolID  SymID         // pointer to the symbol table information
	timeStamp uint64        // time when it is logged
	valueList []interface{} // list of values for the entry in memory
}

// Data in the metadata file file.meta
type metaData struct {
	HeadOffset   uint64 // byte offset to where the next log entry should go
	TailOffset   uint64 // byte offset to start of the the oldest log entry
	MaxSizeBytes uint64 // maximum size in bytes of the data log
	WrapNum      uint64 // wrap number or cycle number
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

// Value list with Go-types into log data ondisk-type list
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

// The file name of the log meta file
func metaFileName(baseFileName string) string {
	return baseFileName + ".met"
}

// LogFileRemove deletes the log file and meta files
func LogFileRemove(baseFileName string) error {
	err := os.Remove(logFileName(baseFileName))
	if err != nil {
		return err
	}
	return os.Remove(metaFileName(baseFileName))
}

// LogFileCreate creates a new log data & meta file and allocates the LogFile struct
func LogFileCreate(baseFileName string, maxFileSizeBytes uint64) (log *LogFile, err error) {

	// create log data file in os
	entryFile, err := os.Create(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}

	// create log meta file in os
	metaFile, err := os.Create(metaFileName(baseFileName))
	if err != nil {
		entryFile.Close()
		return nil, err
	}

	// create the log file structure in memory
	log = &LogFile{
		byteOrder:    binary.LittleEndian,
		entryFile:    entryFile,
		metaFile:     metaFile,
		maxSizeBytes: maxFileSizeBytes,
	}

	// start off the metaFile data
	err = log.updateHeadTail()
	if err != nil {
		return nil, err
	}

	return log, nil
}

// LogFileOpenWrite opens a log file for writing and allocates the LogFile data.
// It needs to position the log at the head of the log file.
// So subsequent writes can append to the head/end of the log file and we can continue
// adding entries.
// Used by a program that creates debug logs or the log server that creates log entries.
func LogFileOpenWrite(baseFileName string) (log *LogFile, err error) {
	log = new(LogFile)
	log.byteOrder = binary.LittleEndian

	// Open for writing from the start of the file
	log.entryFile, err = os.OpenFile(logFileName(baseFileName), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	// Open & read in the meta file
	// Open for writing for updates
	log.metaFile, err = os.OpenFile(metaFileName(baseFileName), os.O_RDWR, 0)
	if err != nil {
		log.entryFile.Close()
		return nil, err
	}
	metaData, err := log.readMetaData()
	if err != nil {
		log.entryFile.Close()
		log.metaFile.Close()
		return nil, err
	}

	// position us at the end of the entry data file
	// seek to head - want to write from head to tail
	_, err = log.entryFile.Seek(int64(metaData.HeadOffset), 0)

	return log, err
}

// LogFileOpenRead opens a log file for reading and allocates the LogFile data
// It needs to position the log at the tail of the log file.
// So subsequent reads can traverse from the tail to the head.
// Used by a program that analyses and looks at debug logs.
func LogFileOpenRead(baseFileName string) (log *LogFile, err error) {
	log = new(LogFile)
	log.byteOrder = binary.LittleEndian

	// Open for reading from the start of the file
	log.entryFile, err = os.Open(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}

	// Open & read in the meta file
	log.metaFile, err = os.Open(metaFileName(baseFileName))
	if err != nil {
		log.entryFile.Close()
		return nil, err
	}
	metaData, err := log.readMetaData()
	if err != nil {
		log.entryFile.Close()
		log.metaFile.Close()
		return nil, err
	}

	// position us at the start of the entry data file
	// seek to tail - want to read from tail to head
	_, err = log.entryFile.Seek(int64(metaData.TailOffset), 0)

	return log, err
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

// Read metadata from the meta file
func (log *LogFile) readMetaData() (data metaData, err error) {
	_, err = log.metaFile.Seek(0, 0)
	if err != nil {
		return data, err
	}
	err = binary.Read(log.metaFile, log.byteOrder, &data)
	return data, err
}

// move the tail to accomodate the new record
// we will have to read log records from the tail to work out
// where the next non overlapping one starts
func (log *LogFile) tailPush(newRecSize uint32) error {

	return nil
}

// updateHeadTail updates the meta file with the head ant tail pointer
// and other relevant log parameters
func (log *LogFile) updateHeadTail() error {
	// where are we in the log data file?
	offset, err := log.entryFile.Seek(0, 1)
	if err != nil {
		return err
	}

	// point to offset in the data file where the next log entry is due to go
	// it is not written there yet but will be there when we next write it
	log.headOffset = uint64(offset)

	data := metaData{HeadOffset: log.headOffset, TailOffset: log.tailOffset, MaxSizeBytes: log.maxSizeBytes, WrapNum: log.wrapNum}
	return log.writeMetaData(data)
}

// inc increments the position of the LSN
func (lsn *LSN) inc() {
	lsn.Pos++
}

// wrap wraps the LSN and resets position
func (lsn *LSN) wrap() {
	lsn.Pos = 0
	lsn.Cycle++
}

// LogFileAddEntry adds an entry to log data file
// This means writing to the file and possibly wrapping around and starting
// from the beginning of the file.
// Note: if an entry would wrap then don't do a partial write but instead start
// from the beginning of the file with the new entry.
func (log *LogFile) LogFileAddEntry(entry LogEntry) error {

	// Wrap case
	// then we would go over the end of the log file
	// so start overwriting the beginning of the file
	if log.headOffset+uint64(entry.SizeBytes()) > log.maxSizeBytes {
		log.headOffset = 0
		log.tailOffset = 0
		log.wrapNum++
		log.entryFile.Seek(0, 0)
		log.nextLogID.wrap()
	}

	// If we have wrapped then
	// we need to update the tail before new record writes over it
	if (log.wrapNum > 0) {
		log.tailPush(entry.SizeBytes())
	}

	entry.logID = log.nextLogID

	// write our data to the file
	_, err := entry.Write(log.entryFile, log.byteOrder)
	if err != nil {
		return err
	}
	//fmt.Printf("len of write entry: %v\n", len)

	// update meta file - head points to next spot to write to
	// unless it is time to wrap to the start again
	err = log.updateHeadTail()
	if err != nil {
		return err
	}

	log.nextLogID.inc()

	//fmt.Printf("log.nextLogID: %v\n", log.nextLogID)
	return nil
}

// SizeBytes returns # of bytes of the entry on disk
func (entry *LogEntry) SizeBytes() uint32 {
	var len uint32
	// sizes of fields on disk
	// 16 - 128 bit LSN
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
//
// It needs the symFile to be read in first so that it can know the types associated
// with a log line (we don't duplicate this information on every instance).
//
// Returns if there is an error or the entry.
// Note: The eof will actually occur when we reach the head of the list and
// not at the end of the real file.
// TODO: work out when we have reached the head and handle when we reach EOF to wrap around.
/*
 * 128 bit LSN
 * 64 bit sym Id
 * 64 bit timestamp
 * 32 bit length of value data
 * ...value data...
 * <types stored in the sym file>
 * type sizes: 8 bit, 32 bit, 64 bit, 32 bit len + len bytes
 */
func (log *LogFile) ReadEntry(sym *SymFile) (entry LogEntry, err error) {
	err = nil
	r := bufio.NewReader(log.entryFile)

	// logID LSN
	err = binary.Read(r, log.byteOrder, &entry.logID)
	if err != nil {
		return entry, err
	}
	//fmt.Printf("read logId: %v\n", entry.logID)

	// symID
	err = binary.Read(r, log.byteOrder, &entry.symbolID)
	if err != nil {
		return entry, err
	}
	//fmt.Printf("symId: %v\n", entry.symbolID)

	// Get typeList from the symbol file
	symEntry, ok := sym.SymFileGetEntry(entry.symbolID)
	if !ok {
		return entry, fmt.Errorf("Can't find symbol in Symbol file")
	}
	keyTypeList := symEntry.keyTypeList
	if err != nil {
		return entry, err
	}

	// timestamp
	err = binary.Read(r, log.byteOrder, &entry.timeStamp)
	if err != nil {
		return entry, err
	}
	//fmt.Printf("timestamp: %v\n", entry.timeStamp)

	// length of value data
	var valLen uint32
	err = binary.Read(r, log.byteOrder, &valLen)
	if err != nil {
		return entry, err
	}

	// If we stored some redudant info about the length of the value liest
	// then we could compare with the Type list length
	// But we don't at the moment:
	// if valListLen != uint32(len(keyTypeList)) {
	// 	return entry, eof, fmt.Errorf("Read value list length %v mismatch with length of symbol type list %v",
	// 		valLen, len(keyTypeList))
	// }

	// valuelist
	entry.valueList, err = readValueList(r, log.byteOrder, keyTypeList)
	if err != nil {
		return entry, err
	}

	return entry, err
}

/*
 * 128 bit LSN
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

// Write all values from valueList to the writer.
// Basic/atomic types are written straight out
// Strings are length prefix encoded
func writeValueList(w io.Writer, byteOrder binary.ByteOrder, valueList []interface{}) (err error) {
	for _, value := range valueList {

		switch value.(type) {
		case uint8, int8, bool, uint32, int32, uint64, int64, float32, float64:
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
			_, err = w.Write([]byte(str))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Read in the value list using the list of types and return the value list
func readValueList(r io.Reader, byteOrder binary.ByteOrder, keyTypeList []KeyType) (valueList []interface{}, err error) {
	valueList = make([]interface{}, 0)
	for i := 0; i < len(keyTypeList); i++ {
		keyType := keyTypeList[i]
		fmt.Printf("reading key value: %v\n", keyType.Key)

		switch keyType.ValueType {
		case TypeUint8:
			var val uint8
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeInt8:
			var val int8
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeInt32:
			var val int32
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeUint32:
			var val uint32
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeInt64:
			var val int64
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeUint64:
			var val uint64
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeFloat32:
			var val float32
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeFloat64:
			var val float64
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeBoolean:
			var val bool
			err = binary.Read(r, byteOrder, &val)
			if err != nil {
				return nil, err
			}
			valueList = append(valueList, val)
		case TypeString:
			var lenHdr uint32
			err = binary.Read(r, byteOrder, &lenHdr)
			if err != nil {
				return nil, err
			}
			b := make([]byte, lenHdr)
			got, err := r.Read(b)
			if err == nil && got < int(lenHdr) {
				err = fmt.Errorf("Only read %d bytes of %d for string", got, lenHdr)
			}
			valueList = append(valueList, string(b))
		case TypeByteData:
			var lenHdr uint32
			err = binary.Read(r, byteOrder, &lenHdr)
			if err != nil {
				return nil, err
			}
			b := make([]byte, lenHdr)
			got, err := r.Read(b)
			if err == nil && got < int(lenHdr) {
				err = fmt.Errorf("Only read %d bytes of %d for byte data", got, lenHdr)
			}
			valueList = append(valueList, b)
		}
		fmt.Printf("%d: valueList = %v\n", i, valueList)
	}
	fmt.Printf("valueList = %v\n", valueList)
	return valueList, err
}
