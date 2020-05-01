package logsym

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"
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
	entryReadFile  *os.File         // file pointer to the entry data for reading
	entryWriteFile *os.File         // file pointer to the entry data for writing
	metaFile       *os.File         // meta file for header type info e.g. head pointer
	reader         *bufio.Reader    // buffered reader from log file
	nextLogID      LSN              // current log sequence number that we are at
	headOffset     uint64           // offset into file where next record goes
	tailOffset     uint64           // offset into file where earliest record is
	wrapNum        uint64           // how many times wrapped
	maxSizeBytes   uint64           // maximum size in bytes that the log record can grow to
	numSizeBytes   uint64           // number of bytes actually used in the file
	NumEntries     uint64           // number of entries in the log
	byteOrder      binary.ByteOrder // the byte ordering of the numbers
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
	NumSizeBytes uint64 // number of bytes being used in file
	NumEntries   uint64 // number of entries in log file
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

// SymbolID returns the log entry's symid link
func (entry LogEntry) SymbolID() SymID {
	return entry.symbolID
}

// TimeStamp returns the associated time stamp with the entry
func (entry LogEntry) TimeStamp() uint64 {
	return entry.timeStamp
}

func tStamp2String(tstamp uint64) string {
	t := time.Unix(0, int64(tstamp))
	return t.Format("02/01/2006, 15:04:05")
}

// StringRelTime returns string version of log entry with relative timestamp from now.
// Useful for testing purposes.
func (entry LogEntry) StringRelTime() string {
	now := uint64(time.Now().UnixNano())
	diff := now - entry.timeStamp
	diffSec := diff / 1000000000

	var str strings.Builder
	fmt.Fprintf(&str, "LogEntry\n")
	fmt.Fprintf(&str, "  logId: %v\n", entry.logID)
	fmt.Fprintf(&str, "  symId: %v\n", entry.symbolID)
	fmt.Fprintf(&str, "  timeStamp rel: now - %v secs\n", diffSec)
	for _, value := range entry.GetValues() {
		fmt.Fprintf(&str, "  value: %v\n", value)
	}

	return str.String()
}

func (entry LogEntry) String() string {

	var str strings.Builder
	fmt.Fprintf(&str, "LogEntry\n")
	fmt.Fprintf(&str, "  logId: %v\n", entry.logID)
	fmt.Fprintf(&str, "  symId: %v\n", entry.symbolID)
	fmt.Fprintf(&str, "  timeStamp: %v (%v)\n", tStamp2String(entry.timeStamp), entry.timeStamp)
	for _, value := range entry.GetValues() {
		fmt.Fprintf(&str, "  value: %v\n", value)
	}

	return str.String()
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

	// create log write data file in os
	entryWriteFile, err := os.Create(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}

	// create log read data file in os
	// this is at start and used to move the tail onwards
	entryReadFile, err := os.Open(logFileName(baseFileName))
	if err != nil {
		entryWriteFile.Close()
		return nil, err
	}
	reader := bufio.NewReader(entryReadFile)

	// create log meta file in os
	metaFile, err := os.Create(metaFileName(baseFileName))
	if err != nil {
		entryReadFile.Close()
		entryWriteFile.Close()
		return nil, err
	}

	// create the log file structure in memory
	log = &LogFile{
		byteOrder:      binary.LittleEndian,
		entryWriteFile: entryWriteFile,
		entryReadFile:  entryReadFile,
		reader:         reader,
		metaFile:       metaFile,
		maxSizeBytes:   maxFileSizeBytes,
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
	log.entryWriteFile, err = os.OpenFile(logFileName(baseFileName), os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			log.entryWriteFile.Close()
		}
	}()

	// Open for reading the tail entries as we push
	log.entryReadFile, err = os.OpenFile(logFileName(baseFileName), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			log.entryReadFile.Close()
		}
	}()

	// Open & read in the meta file
	// Open for writing for updates
	log.metaFile, err = os.OpenFile(metaFileName(baseFileName), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			log.metaFile.Close()
		}
	}()

	metaData, err := log.readMetaData()
	if err != nil {
		return nil, err
	}

	// position us at the end of the entry data file
	// seek to head - want to write from head to tail
	_, err = log.entryWriteFile.Seek(int64(metaData.HeadOffset), 0)
	if err != nil {
		return nil, err
	}

	// Get the tail ready for reading so we can push the tail as we write a record
	_, err = log.entryReadFile.Seek(int64(metaData.TailOffset), 0)
	if err != nil {
		return nil, err
	}

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
	log.entryReadFile, err = os.Open(logFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			log.entryReadFile.Close()
		}
	}()

	// Open & read in the meta file
	log.metaFile, err = os.Open(metaFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			log.metaFile.Close()
		}
	}()

	metaData, err := log.readMetaData()
	if err != nil {
		return nil, err
	}
	
	// ???? where do we update the log from the metadata???

	// position us at the start of the entry data file
	// seek to tail - want to read from tail to head
	stdlog.Printf("seek to tail: %v\n", metaData.TailOffset)
	_, err = log.entryReadFile.Seek(int64(metaData.TailOffset), 0)

	log.reader = bufio.NewReader(log.entryReadFile)

	return log, err
}

func (log LogFile) String() string {
	var str strings.Builder
	fmt.Fprintf(&str, "LogFile\n")
	fmt.Fprintf(&str, "  nextLogId: %v\n", log.nextLogID)
	fmt.Fprintf(&str, "  headOffset: %v\n", log.headOffset)
	fmt.Fprintf(&str, "  tailOffset: %v\n", log.tailOffset)
	fmt.Fprintf(&str, "  wrapNum: %v\n", log.wrapNum)
	fmt.Fprintf(&str, "  numSizeBytes: %v\n", log.numSizeBytes)
	fmt.Fprintf(&str, "  maxSizeBytes: %v\n", log.maxSizeBytes)
	fmt.Fprintf(&str, "  byteOrder: %v\n", log.byteOrder)

	return str.String()
}

// LogFileClose closes the associate files
func (log *LogFile) LogFileClose() error {
	var err1, err2, err3 error

	if log.entryWriteFile != nil {
		err1 = log.entryWriteFile.Close()
	}
	if log.entryReadFile != nil {
		err2 = log.entryReadFile.Close()
	}
	if log.metaFile != nil {
		err3 = log.metaFile.Close()
	}

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
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

// 1st lap - no tail pushing needed
//   |-----|-----|------|------|.......|
//   ^     ^                           ^
//   tail  head                        maxsize
//
// Going into 2nd lap
//   |-----|-----|------|------|....|......|
//   ^                              ^      ^
//   tail                           head   maxsize
//   Add 1 more rec but it won't fit
//   |-----|-----|------|------|....|000000|
//   ^                              ^      ^
//   tail                           head   maxsize
//   if we can't fit then record entry starts at beginning of file
//   and we push the tail from there.

// We are trying to write to where the head is but there
// may not be enough room because we bump into the tail.
// So move the tail to accommodate the new record
// we will have to read log records from the tail to work out
// where the next non overlapping one starts
// A record does not span around the end of the file -
// it will move to the start of the file if necessary.
func (log *LogFile) tailPush(sym *SymFile, newRecSize uint32) error {

	// lap >= 2
	//   |-----|-----|------|------|.......|
	//         ^     ^                     ^
	//        head   tail                  maxsize
	//         |-gap-|
	// loop reading in records from the tail
	// until the accumulated size (including head-tail gap is greater than the new rec size
	// tail points to oldest complete record

	tailGap := log.tailOffset - log.headOffset
	sizeAvailable := tailGap
	stdlog.Printf("Tail push for new rec size of %v - avail of %v\n", newRecSize, sizeAvailable)
	if tailGap >= 0 { // tail in front of head
	    // set read pos to the tail
		log.entryReadFile.Seek(int64(log.tailOffset), 0)
		for {
			if uint64(newRecSize) <= sizeAvailable {
				// moved tail far enough and we have space for a new record
				stdlog.Printf("Moved tail far enough. available=%v, newRecSize=%v\n", sizeAvailable, newRecSize)
				return nil
			}
			// TODO:
			// read entry from tail - so need to set read file pos to the tail offset
			entry, err := log.ReadEntryData(sym)
			if err == nil {
				reclen := entry.SizeBytes()
				sizeAvailable += uint64(reclen)  // size engulfs old tail record
				log.tailOffset += uint64(reclen) // tail moves forward to newer record
				log.NumEntries--
				stdlog.Printf("Move tail over 1 record, tail=%v, avail=%v, numRecs=%v\n", log.tailOffset, sizeAvailable, log.NumEntries)
			} else if err == io.EOF {
				// TODO: why don't we continue tail pushing after wrapping??????
				stdlog.Printf("We hit EOF, no more tail entries to read\n")
				// we hit the end and no more tail entries to read
				// BUT if there is a gap at the end it might be
				// big enough for the head entry
				// compare tailOffset with maxsize
				// |------|000000000|
				// ^      ^---------^
				//        tail      maxsize
				endGap := log.maxSizeBytes - log.tailOffset
				if uint64(newRecSize) <= sizeAvailable+endGap {
					// then fit in the end gap
					stdlog.Printf("Fit into end gap\n")
					sizeAvailable += endGap
				} else {
					// zero out where head is and move head around
					stdlog.Printf("Zero our where head is and move head around to the start\n")
					sizeAvailable = 0
					log.headOffset = 0
					log.wrapNum++
					log.setWriteZeroPos()
					log.nextLogID.wrap()
				}
				log.tailOffset = 0 // wrap around tail
				_, err = log.setReadZeroPos()
				if err != nil {
					return err
				}
			} else {
				return err
			}
		} // for each tail record
	} // if
	return nil
}

func (log *LogFile) getWritePos() (int64, error) {
	return log.entryWriteFile.Seek(0, 1)
}

func (log *LogFile) getReadPos() (int64, error) {
	return log.entryReadFile.Seek(0, 1)
}

func (log *LogFile) setReadZeroPos() (int64, error) {
	return log.entryReadFile.Seek(0, 0)
}

func (log *LogFile) setWriteZeroPos() (int64, error) {
	return log.entryWriteFile.Seek(0, 0)
}

// updateHeadTail updates the meta file with the head ant tail pointer
// and other relevant log parameters
func (log *LogFile) updateHeadTail() error {
	// where are we in the log data file?
	offset, err := log.getWritePos()
	if err != nil {
		return err
	}

	// point to offset in the data file where the next log entry is due to go
	// it is not written there yet but will be there when we next write it
	log.headOffset = uint64(offset)

	stdlog.Printf("Update head %v and tail %v\n", log.headOffset, log.tailOffset)
	data := metaData{HeadOffset: log.headOffset, TailOffset: log.tailOffset, MaxSizeBytes: log.maxSizeBytes, 
		NumSizeBytes: log.numSizeBytes, WrapNum: log.wrapNum, NumEntries: log.NumEntries}
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
func (log *LogFile) LogFileAddEntry(sym *SymFile, entry LogEntry) error {
	stdlog.Printf("Adding log entry=%v\n", entry)

	// Wrap case on 1st iteration
	// then we would go over the end of the log file
	// so start overwriting the beginning of the file
	if log.wrapNum == 0 && log.headOffset+uint64(entry.SizeBytes()) > log.maxSizeBytes {
		stdlog.Printf("Do an internal log wrap\n")
		
		// TODO: need to write some marker at the end of the last record or know we are at EOF
		// ie. at EOF or have filler space upto EOF
		// Do we mark last valid record in any way different or just zero data at end?
		// Note the highest point after valid data.
		log.numSizeBytes = log.headOffset
		
		log.headOffset = 0
		log.tailOffset = 0
		log.wrapNum++
		log.setWriteZeroPos()
		log.nextLogID.wrap()
	}

	// If we have wrapped then
	// we need to update the tail before new record writes over it
	if log.wrapNum > 0 {
		log.tailPush(sym, entry.SizeBytes())
	}

	// Now have room for new entry to go at the head and tail is out of way

	entry.logID = log.nextLogID

	// write our data to the file
	_, err := entry.Write(log.entryWriteFile, log.byteOrder)
	if err != nil {
		return err
	}
	
	log.NumEntries++
	stdlog.Printf("len of write entry: %v, numRecs: %v\n", entry.SizeBytes(), log.NumEntries)

	// update meta file - head points to next spot to write to
	// unless it is time to wrap to the start again
	err = log.updateHeadTail()
	if err != nil {
		return err
	}

	log.nextLogID.inc()

	stdlog.Printf("log.nextLogID: %v\n", log.nextLogID)
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

func (log *LogFile) validZone(offset uint64) bool {
	// Read from Tail to Head
    if log.headOffset > log.tailOffset {
    	// T.....o.....H
    	// read left to right from tail to head
        return log.tailOffset <= offset && offset <= log.tailOffset 
	} else if log.headOffset < log.tailOffset {
		// ....H     T......
		// read left to right until hit EOF and then from start to the head
		return offset <= log.headOffset || offset>= log.tailOffset
	} else {
        // .......TH.......		
        // valid everywhere but stop when get to head a 2nd time
        // offset back to where we started
        // Idea: Check if seen element before and if so then stop
        // So do checking outside of read
        return true
	}
}


/*
 * Wrapper around ReadEntryData
 * Check if we reached the end when we reach the HEAD
 * Check if we reached the end of file when reached the size of file.
 */
func (log *LogFile) ReadEntry(sym *SymFile) (entry LogEntry, err error) {
	offset, err := log.getReadPos()
	if err != nil {
		return entry, err
	}
	
	// Need to check if reached the head
	// in which case we need to stop
	if !log.validZone(uint64(offset)) {
	    stdlog.Printf("offset = %v, head = %v\n", offset, log.headOffset)	
		return entry, io.EOF
	}

	// Need to check if reached end of viable data
	// then need to wrap to start of file
	if uint64(offset) >= log.numSizeBytes {
	    // wrap to start	
	    _, err = log.setReadZeroPos()
		if err != nil {
			return entry, err
		}
	}

	return log.ReadEntryData(sym)
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
func (log *LogFile) ReadEntryData(sym *SymFile) (entry LogEntry, err error) {
	err = nil

	// logID LSN
	err = binary.Read(log.reader, log.byteOrder, &entry.logID)
	if err != nil {
		return entry, err
	}
	stdlog.Printf("read logId: %v\n", entry.logID)

	// symID
	err = binary.Read(log.reader, log.byteOrder, &entry.symbolID)
	if err != nil {
		return entry, err
	}
	stdlog.Printf("symId: %v\n", entry.symbolID)

	// Get typeList from the symbol file
	symEntry, ok := sym.SymFileGetEntry(entry.symbolID)
	if !ok {
		return entry, fmt.Errorf("Can't find symbol in Symbol file: %d", entry.symbolID)
	}
	keyTypeList := symEntry.keyTypeList
	if err != nil {
		return entry, err
	}

	// timestamp
	err = binary.Read(log.reader, log.byteOrder, &entry.timeStamp)
	if err != nil {
		return entry, err
	}
	stdlog.Printf("timestamp: %v\n", entry.timeStamp)

	// length of value data
	var valLen uint32
	err = binary.Read(log.reader, log.byteOrder, &valLen)
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
	entry.valueList, err = readValueList(log.reader, log.byteOrder, keyTypeList)
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
	stdlog.Printf("logId: %v\n", entry.logID)

	// symID
	length += binary.Size(entry.symbolID)
	err = binary.Write(w, byteOrder, entry.symbolID)
	if err != nil {
		return 0, err
	}
	stdlog.Printf("symId: %v\n", entry.symbolID)

	// timestamp
	length += binary.Size(entry.timeStamp)
	err = binary.Write(w, byteOrder, entry.timeStamp)
	if err != nil {
		return 0, err
	}
	stdlog.Printf("timestamp: %v\n", entry.timeStamp)

	// length of value data
	length += 4
	valLen := sizeOfValues(entry.valueList)
	err = binary.Write(w, byteOrder, valLen)
	if err != nil {
		return 0, err
	}
	stdlog.Printf("value len: %v\n", valLen)

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
		stdlog.Printf("reading key value: %v\n", keyType.Key)

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
		stdlog.Printf("%d: valueList = %v\n", i, valueList)
	}
	stdlog.Printf("valueList = %v\n", valueList)
	return valueList, err
}
