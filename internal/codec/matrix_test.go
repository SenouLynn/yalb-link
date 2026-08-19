package codec

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// matrixPath is the capability matrix this package is checked against.
const matrixPath = "../../docs/reference/codec-capability-matrix.md"

// Matrix rows look like:
//
//	| RECV-ATTITUDE | ATTITUDE | 30 | common | Decode → Attitude | fixture | complete | |
//
// The ID column is what ties a row to a dispatch table key. Rows without a
// numeric ID column (the NORM-* and FRAME-* tables) are matched by case ID
// prefix instead and are not part of the dispatch comparison.
var matrixRow = regexp.MustCompile(`^\|\s*(RECV|SEND)-([A-Z0-9-]+)\s*\|[^|]*\|\s*(\d+)\s*\|`)

// TestMatrixCoverage asserts the capability matrix and the dispatch tables
// describe the same codec.
//
// This is the layer that stops the matrix being a wish list. check-matrix.sh
// only proves nobody typed "unstarted"; it cannot tell whether a row
// corresponds to code. Here a decoder with no matrix row fails, and a matrix
// row with no decoder fails.
func TestMatrixCoverage(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("reading capability matrix: %v", err)
	}

	recvRows := map[uint32]string{}
	sendRows := map[uint32]string{}

	for _, line := range strings.Split(string(raw), "\n") {
		m := matrixRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		id, convErr := strconv.ParseUint(m[3], 10, 32)
		if convErr != nil {
			t.Errorf("row %s-%s has unparseable message ID %q", m[1], m[2], m[3])

			continue
		}

		caseID := m[1] + "-" + m[2]

		if m[1] == "RECV" {
			recvRows[uint32(id)] = caseID
		} else {
			sendRows[uint32(id)] = caseID
		}
	}

	// Receive: the union of both decoder tables.
	decoderIDs := map[uint32]bool{}

	for id := range telemetryDecoders {
		decoderIDs[id] = true
	}

	for id := range protocolDecoders {
		decoderIDs[id] = true
	}

	compare(t, "receive", keysOf(decoderIDs), keysOfStr(recvRows),
		"decoder in telemetryDecoders/protocolDecoders", "RECV row in the matrix")

	// Send: the declared encoder families.
	sendIDs := map[uint32]bool{}

	for _, id := range SendFamilies {
		sendIDs[id] = true
	}

	compare(t, "send", keysOf(sendIDs), keysOfStr(sendRows),
		"entry in SendFamilies", "SEND row in the matrix")
}

// compare reports IDs present on one side and missing on the other.
func compare(t *testing.T, label string, code, matrix []uint32, codeDesc, matrixDesc string) {
	t.Helper()

	inMatrix := map[uint32]bool{}
	for _, id := range matrix {
		inMatrix[id] = true
	}

	inCode := map[uint32]bool{}
	for _, id := range code {
		inCode[id] = true
	}

	for _, id := range code {
		if !inMatrix[id] {
			t.Errorf("%s: message ID %d has a %s but no %s", label, id, codeDesc, matrixDesc)
		}
	}

	for _, id := range matrix {
		if !inCode[id] {
			t.Errorf("%s: message ID %d has a %s but no %s", label, id, matrixDesc, codeDesc)
		}
	}
}

func keysOf(m map[uint32]bool) []uint32 {
	out := make([]uint32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}

func keysOfStr(m map[uint32]string) []uint32 {
	out := make([]uint32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}

// The matrix must claim exactly the family counts the roadmap fixed.
func TestMatrixFamilyCounts(t *testing.T) {
	t.Parallel()

	if got := len(telemetryDecoders); got != 13 {
		t.Errorf("telemetryDecoders has %d families, want 13", got)
	}

	if got := len(protocolDecoders); got != 5 {
		t.Errorf("protocolDecoders has %d families, want 5", got)
	}

	if got := len(SendFamilies); got != 11 {
		t.Errorf("SendFamilies has %d families, want 11", got)
	}
}
