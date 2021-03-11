package verifier

/*

#cgo CFLAGS: -I${SRCDIR}/../../../../.pact-go-libs/include -g -Wall
#cgo LDFLAGS: -L${SRCDIR}/../../../../.pact-go-libs -v
#cgo darwin,amd64 LDFLAGS: -lpact_verifier_ffi -Wl,-framework,Security

// #cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../../libs/libpact_verifier_ffi.a

// Library headers
typedef int bool;
#define true 1
#define false 0

void init(char* log);
char* version();
void free_string(char* s);
int verify(char* s);

*/
import "C"
