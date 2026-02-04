# MJPEG Trigger Test Suite

This document describes the comprehensive test suite for the MJPEG trigger implementation.

## Test Files

### 1. event_test.go
Tests the Event struct and all its methods implementing the nuclio.Event interface.

**Test Coverage:**
- `TestEvent`: Tests all Event getter methods
  - `GetBody()` - Returns frame data
  - `GetBodyString()` - Returns frame data as string
  - `GetBodyObject()` - Returns nil (binary data)
  - `GetPath()` - Returns MJPEG stream URL
  - `GetURL()` - Returns MJPEG stream URL
  - `GetMethod()` - Returns empty string
  - `GetShardID()` - Returns 0
  - `GetType()` - Returns "mjpeg"
  - `GetTypeVersion()` - Returns empty string
  - `GetTimestamp()` - Returns frame timestamp
  - `GetContentType()` - Returns "image/jpeg"
  - `GetSize()` - Returns frame data size
  - `GetHeaders()` - Returns nil
  - `GetHeader()` - Returns nil for any key
  - `GetHeaderByteSlice()` - Returns nil
  - `GetHeaderString()` - Returns empty string
  - `GetHeaderInt()` - Returns 0
  - `GetFields()` - Returns map with frame_num, url, timestamp
  - `GetField()` - Returns field value by name
  - `GetFieldString()` - Returns string fields
  - `GetFieldByteSlice()` - Returns byte slice fields
  - `GetFieldInt()` - Returns int fields

- `TestEventWithDifferentData`: Tests Event with various data sizes
  - Large frames (1MB)
  - Empty frames
  - Nil frames

**Total Test Cases:** 25

### 2. types_test.go
Tests the Configuration struct and validation logic.

**Test Coverage:**
- `TestTypes/NewConfiguration_Success` - Valid configuration
- `TestTypes/NewConfiguration_DefaultProcessingFactor` - Default value applied
- `TestTypes/NewConfiguration_MissingURL` - Error when URL missing
- `TestTypes/NewConfiguration_InvalidProcessingFactor` - Error when factor = 0
- `TestTypes/NewConfiguration_NegativeProcessingFactor` - Error when factor < 0
- `TestTypes/NewConfiguration_ProcessingFactor10` - Valid high factor value

**Total Test Cases:** 6

### 3. factory_test.go
Tests the trigger factory registration.

**Test Coverage:**
- `TestFactory/FactoryRegistration` - Verifies "mjpeg" trigger is registered in trigger registry

**Total Test Cases:** 1

### 4. trigger_test.go
Tests the core trigger implementation methods.

**Test Coverage:**
- `TestExtractBoundary` - Tests boundary parsing from Content-Type header
  - Standard format: `boundary=myboundary`
  - With spaces: `boundary = myboundary`
  - Missing boundary
  - Empty content type

- `TestReadHeaders` - Tests HTTP header parsing from MJPEG stream
  - Parses Content-Type
  - Parses Content-Length
  
- `TestGetContentLength` - Tests content length extraction
  - Standard "Content-Length" header
  - Lowercase "content-length" header
  - Missing header
  - Invalid value

- `TestReadUntil` - Tests boundary delimiter detection in stream

- `TestGetConfig` - Tests configuration retrieval

**Total Test Cases:** 12

## Test Execution

Run all tests:
```bash
go test -v ./pkg/processor/trigger/mjpeg/...
```

Run specific test file:
```bash
go test -v ./pkg/processor/trigger/mjpeg/event_test.go
go test -v ./pkg/processor/trigger/mjpeg/types_test.go
go test -v ./pkg/processor/trigger/mjpeg/factory_test.go
go test -v ./pkg/processor/trigger/mjpeg/trigger_test.go
```

Run with coverage:
```bash
go test -cover ./pkg/processor/trigger/mjpeg/...
```

## Test Summary

**Total Test Files:** 4
**Total Test Cases:** 44
**Test Coverage Areas:**
- ✅ Event interface implementation (all 20+ methods)
- ✅ Configuration validation
- ✅ Factory registration
- ✅ MJPEG stream parsing (boundary, headers, content-length)
- ✅ Edge cases (empty data, large data, invalid inputs)

## Integration Testing

For integration testing with an actual MJPEG stream, you can:

1. Set up a test MJPEG server (see `test/extproc/` for reference patterns)
2. Use existing MJPEG camera streams for manual testing
3. Create mock HTTP server that streams multipart JPEG data

Example test MJPEG URLs:
- Public webcams providing MJPEG streams
- Local test server: `python -m http.server` with custom MJPEG generator
- Docker containers with MJPEG streaming capabilities
