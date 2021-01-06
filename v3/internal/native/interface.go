// Package native contains the c bindings into the Pact Reference types.
package native

/*
// Library headers
#include <stdlib.h>
typedef int bool;
#define true 1
#define false 0

void init(char* log);
int create_mock_server(char* pact, char* addr, bool tls);
int mock_server_matched(int port);
char* mock_server_mismatches(int port);
bool cleanup_mock_server(int port);
int write_pact_file(int port, char* dir);
int plugin_write_pact_file(int port, char* dir);
void free_string(char* s);
char* get_tls_ca_certificate();
int create_plugin_mock_server(int port, char* config);
int add_plugin_interaction(int port, char* config);
char* plugin_mock_server_mismatches(int port);
int plugin_mock_server_matched(int port);
bool cleanup_plugin_mock_server(int port);

*/
import "C"

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"unsafe"
)

// Request is the sub-struct of Mismatch
type Request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// [
//   {
//     "method": "GET",
//     "mismatches": [
//       {
//         "actual": "",
//         "expected": "\"Bearer 1234\"",
//         "key": "Authorization",
//         "mismatch": "Expected header 'Authorization' but was missing",
//         "type": "HeaderMismatch"
//       }
//     ],
//     "path": "/foobar",
//     "type": "request-mismatch"
//   }
// ]

// MismatchDetail contains the specific assertions that failed during the verification
type MismatchDetail struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
	Key      string `json:"key"`
	Mismatch string `json:"mismatch"`
	Type     string `json:"type"`
}

// MismatchedRequest contains details of any request mismatches during pact verification
type MismatchedRequest struct {
	Request
	Mismatches []MismatchDetail `json:"mismatches"`
	Type       string           `json:"type"`
}

// PluginMismatchDetail contains the specific interaction level mismatch
type PluginInteractionMismatchDetail struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
	Mismatch string `json:"mismatch"`
}

// PluginMismatchRequest contains details of any request mismatches during pact verification
type PluginInteractionMismatch struct {
	Mismatches []PluginInteractionMismatchDetail `json:"mismatches"`
}

// Init initialises the library
func Init() {
	log.Println("[DEBUG] initialising rust mock server interface")
	logLevel := C.CString("LOG_LEVEL")
	defer free(logLevel)

	C.init(logLevel)
}

// CreateMockServer creates a new Mock Server from a given Pact file.
// Returns the port number it started on or an error if failed
func CreateMockServer(pact string, address string, tls bool) (int, error) {
	log.Println("[DEBUG] mock server starting on address:", address)
	cPact := C.CString(pact)
	cAddress := C.CString(address)
	defer free(cPact)
	defer free(cAddress)
	tlsEnabled := 0
	if tls {
		tlsEnabled = 1
	}

	p := C.create_mock_server(cPact, cAddress, C.int(tlsEnabled))

	// | Error | Description |
	// |-------|-------------|
	// | -1 | A null pointer was received |
	// | -2 | The pact JSON could not be parsed |
	// | -3 | The mock server could not be started |
	// | -4 | The method panicked |
	// | -5 | The address is not valid |
	// | -6 | Could not create the TLS configuration with the self-signed certificate |
	port := int(p)
	switch port {
	case -1:
		return 0, ErrInvalidMockServerConfig
	case -2:
		return 0, ErrInvalidPact
	case -3:
		return 0, ErrMockServerUnableToStart
	case -4:
		return 0, ErrMockServerPanic
	case -5:
		return 0, ErrInvalidAddress
	case -6:
		return 0, ErrMockServerTLSConfiguration
	default:
		if port > 0 {
			log.Println("[DEBUG] mock server running on port:", port)
			return port, nil
		}
		return port, fmt.Errorf("an unknown error (code: %v) occurred when starting a mock server for the test", port)
	}
}

// CreatePluginMockServer starts a mock server from a plugin provider
func CreatePluginMockServer(port int, cmd string) (int, error) {
	log.Println("[DEBUG] starting plugin server")
	cCmd := C.CString(cmd)
	defer free(cCmd)
	p := C.create_plugin_mock_server(C.int(port), cCmd)

	return int(p), nil
}

// AddPluginInteractions starts a mock server from a plugin provider
func AddPluginInteractions(port int, interactions []interface{}) error {
	log.Println("[DEBUG] plugin interface adding interactions", interactions)
	payload := struct {
		Interactions []interface{} `json:"interactions"`
	}{
		Interactions: interactions,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		log.Println("[ERROR] unable to serialise interactions to JSON:", err)
		return err
	}
	log.Println("[DEBUG] adding interactions", string(b))
	cInteractions := C.CString(string(b))
	defer free(cInteractions)
	C.add_plugin_interaction(C.int(port), cInteractions)

	// TODO: handle error (parse int above)
	return nil
}

// CleanupPluginMockServer frees the memory from the previous mock server.
func CleanupPluginMockServer(port int) bool {
	log.Println("[DEBUG] plugin mock server cleaning up port:", port)
	res := C.cleanup_plugin_mock_server(C.int(port))

	return int(res) == 1
}

// Verify verifies that all interactions were successful. If not, returns a slice
// of Mismatch-es. Does not write the pact or cleanup server.
func Verify(port int) (bool, []MismatchedRequest) {
	res := C.mock_server_matched(C.int(port))

	mismatches := MockServerMismatchedRequests(port)
	log.Println("[DEBUG] mock server mismatches:", len(mismatches))

	return int(res) == 1, mismatches
}

// VerifyPlugin verifies that all interactions were successful. If not, returns a slice
// of Mismatch-es. Does not write the pact or cleanup server.
func VerifyPlugin(port int) (bool, PluginInteractionMismatch) {
	log.Println("[DEBUG] VerifyPlugin():", port)
	res := C.plugin_mock_server_matched(C.int(port))

	mismatched, err := PluginMockServerMismatchedRequests(port)
	if err != nil {
		log.Println("[ERROR] error parsing response from FFI", err)
		return false, PluginInteractionMismatch{}
	}

	log.Println("[DEBUG] plugin mock server mismatches:", len(mismatched.Mismatches))

	return int(res) == 1, mismatched
}

// PluginMockServerMismatchedRequests returns a JSON object containing any mismatches from
// the last set of interactions for a plugin server
func PluginMockServerMismatchedRequests(port int) (PluginInteractionMismatch, error) {
	log.Println("[DEBUG] mock server determining mismatches:", port)
	var res PluginInteractionMismatch

	mismatches := C.plugin_mock_server_mismatches(C.int(port))
	err := json.Unmarshal([]byte(C.GoString(mismatches)), &res)

	if err != nil {
		return PluginInteractionMismatch{}, err
	}

	return res, nil
}

// MockServerMismatchedRequests returns a JSON object containing any mismatches from
// the last set of interactions.
func MockServerMismatchedRequests(port int) []MismatchedRequest {
	log.Println("[DEBUG] mock server determining mismatches:", port)
	var res []MismatchedRequest

	mismatches := C.mock_server_mismatches(C.int(port))
	json.Unmarshal([]byte(C.GoString(mismatches)), &res)

	return res
}

// CleanupMockServer frees the memory from the previous mock server.
func CleanupMockServer(port int) bool {
	log.Println("[DEBUG] mock server cleaning up port:", port)
	res := C.cleanup_mock_server(C.int(port))

	return int(res) == 1
}

var (
	// ErrMockServerPanic indicates a panic ocurred when invoking the remote Mock Server.
	ErrMockServerPanic = fmt.Errorf("a general panic occured when starting/invoking mock service (this indicates a defect in the framework)")

	// ErrUnableToWritePactFile indicates an error when writing the pact file to disk.
	ErrUnableToWritePactFile = fmt.Errorf("unable to write to file")

	// ErrMockServerNotfound indicates the Mock Server could not be found.
	ErrMockServerNotfound = fmt.Errorf("unable to find mock server with the given port")

	// ErrInvalidMockServerConfig indicates an issue configuring the mock server
	ErrInvalidMockServerConfig = fmt.Errorf("configuration for the mock server was invalid and an unknown error occurred (this is most likely a defect in the framework)")

	// ErrInvalidPact indicates the pact file provided to the mock server was not a valid pact file
	ErrInvalidPact = fmt.Errorf("pact given to mock server is invalid")

	// ErrMockServerUnableToStart means the mock server could not be started in the rust library
	ErrMockServerUnableToStart = fmt.Errorf("unable to start the mock server")

	// ErrInvalidAddress means the address provided to the mock server was invalid and could not be understood
	ErrInvalidAddress = fmt.Errorf("invalid address provided to the mock server")

	// ErrMockServerTLSConfiguration indicates a TLS mock server could not be started
	// and is likely a framework level problem
	ErrMockServerTLSConfiguration = fmt.Errorf("a tls mock server could not be started (this is likely a defect in the framework)")
)

// WritePactFile writes the Pact to file.
func WritePactFile(port int, dir string) error {
	log.Println("[DEBUG] writing pact file for mock server on port:", port, ", dir:", dir)
	cDir := C.CString(dir)
	defer free(cDir)

	res := int(C.write_pact_file(C.int(port), cDir))

	// | Error | Description |
	// |-------|-------------|
	// | 1 | A general panic was caught |
	// | 2 | The pact file was not able to be written |
	// | 3 | A mock server with the provided port was not found |
	switch res {
	case 0:
		return nil
	case 1:
		return ErrMockServerPanic
	case 2:
		return ErrUnableToWritePactFile
	case 3:
		return ErrMockServerNotfound
	default:
		return fmt.Errorf("an unknown error ocurred when writing to pact file")
	}
}

// GetTLSConfig returns a tls.Config compatible with the TLS
// mock server
func GetTLSConfig() *tls.Config {
	cert := C.get_tls_ca_certificate()
	defer libRustFree(cert)

	goCert := C.GoString(cert)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM([]byte(goCert))

	return &tls.Config{
		RootCAs: certPool,
	}
}

func free(str *C.char) {
	C.free(unsafe.Pointer(str))
}

func libRustFree(str *C.char) {
	C.free_string(str)
}
