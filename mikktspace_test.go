package mikktspace

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// ─── minimal inline glTF/GLB reader ───────────────────────────────────────────

type glbMesh struct {
	positions []float32 // flat VEC3
	normals   []float32 // flat VEC3
	texcoords []float32 // flat VEC2
	indices   []uint32
}

// gltfJSON holds just enough of the glTF JSON to locate accessor data.
type gltfJSON struct {
	Accessors   []gltfAccessor   `json:"accessors"`
	BufferViews []gltfBufferView `json:"bufferViews"`
	Meshes      []gltfMesh       `json:"meshes"`
}

type gltfAccessor struct {
	BufferView    int    `json:"bufferView"`
	ByteOffset    int    `json:"byteOffset"`
	ComponentType int    `json:"componentType"`
	Count         int    `json:"count"`
	Type          string `json:"type"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type gltfMesh struct {
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    *int           `json:"indices"`
}

func componentSize(compType int) int {
	switch compType {
	case 5120, 5121: // BYTE, UNSIGNED_BYTE
		return 1
	case 5122, 5123: // SHORT, UNSIGNED_SHORT
		return 2
	case 5125: // UNSIGNED_INT
		return 4
	case 5126: // FLOAT
		return 4
	}
	return 0
}

func numComponents(typ string) int {
	switch typ {
	case "SCALAR":
		return 1
	case "VEC2":
		return 2
	case "VEC3":
		return 3
	case "VEC4":
		return 4
	}
	return 0
}

func readFloats(bin []byte, acc gltfAccessor, bv gltfBufferView, nComp int) []float32 {
	stride := bv.ByteStride
	elemSize := nComp * 4 // float32 = 4 bytes
	if stride == 0 {
		stride = elemSize
	}
	base := bv.ByteOffset + acc.ByteOffset
	out := make([]float32, acc.Count*nComp)
	for i := 0; i < acc.Count; i++ {
		off := base + i*stride
		for c := 0; c < nComp; c++ {
			out[i*nComp+c] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+c*4:]))
		}
	}
	return out
}

func readIndices(bin []byte, acc gltfAccessor, bv gltfBufferView) []uint32 {
	stride := bv.ByteStride
	cs := componentSize(acc.ComponentType)
	if stride == 0 {
		stride = cs
	}
	base := bv.ByteOffset + acc.ByteOffset
	out := make([]uint32, acc.Count)
	for i := 0; i < acc.Count; i++ {
		off := base + i*stride
		switch acc.ComponentType {
		case 5121: // uint8
			out[i] = uint32(bin[off])
		case 5123: // uint16
			out[i] = uint32(binary.LittleEndian.Uint16(bin[off:]))
		case 5125: // uint32
			out[i] = binary.LittleEndian.Uint32(bin[off:])
		}
	}
	return out
}

func loadGLB(path string) (*glbMesh, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// GLB header
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x46546C67 {
		return nil, fmt.Errorf("not a GLB file (magic=%08x)", magic)
	}

	// JSON chunk
	jsonLen := binary.LittleEndian.Uint32(data[12:16])
	jsonType := binary.LittleEndian.Uint32(data[16:20])
	if jsonType != 0x4E4F534A {
		return nil, fmt.Errorf("expected JSON chunk, got %08x", jsonType)
	}
	jsonData := data[20 : 20+jsonLen]

	// BIN chunk
	binOff := 20 + jsonLen
	// Align to 4 bytes (GLB spec requires chunk alignment)
	if binOff%4 != 0 {
		binOff += 4 - (binOff % 4)
	}
	binLen := binary.LittleEndian.Uint32(data[binOff : binOff+4])
	binType := binary.LittleEndian.Uint32(data[binOff+4 : binOff+8])
	if binType != 0x004E4942 {
		return nil, fmt.Errorf("expected BIN chunk, got %08x", binType)
	}
	bin := data[binOff+8 : binOff+8+binLen]

	// Parse JSON
	var gltf gltfJSON
	if err := json.Unmarshal(jsonData, &gltf); err != nil {
		return nil, fmt.Errorf("JSON parse: %w", err)
	}

	// Find first primitive
	if len(gltf.Meshes) == 0 || len(gltf.Meshes[0].Primitives) == 0 {
		return nil, fmt.Errorf("no mesh primitives found")
	}
	prim := gltf.Meshes[0].Primitives[0]

	mesh := &glbMesh{}

	// Positions
	if idx, ok := prim.Attributes["POSITION"]; ok {
		acc := gltf.Accessors[idx]
		bv := gltf.BufferViews[acc.BufferView]
		mesh.positions = readFloats(bin, acc, bv, 3)
	}

	// Normals
	if idx, ok := prim.Attributes["NORMAL"]; ok {
		acc := gltf.Accessors[idx]
		bv := gltf.BufferViews[acc.BufferView]
		mesh.normals = readFloats(bin, acc, bv, 3)
	}

	// Texcoords
	if idx, ok := prim.Attributes["TEXCOORD_0"]; ok {
		acc := gltf.Accessors[idx]
		bv := gltf.BufferViews[acc.BufferView]
		mesh.texcoords = readFloats(bin, acc, bv, 2)
	}

	// Indices
	if prim.Indices != nil {
		acc := gltf.Accessors[*prim.Indices]
		bv := gltf.BufferViews[acc.BufferView]
		mesh.indices = readIndices(bin, acc, bv)
	}

	return mesh, nil
}

// ─── golden comparison ────────────────────────────────────────────────────────

type goldenEntry struct {
	Face                  int        `json:"face"`
	Vert                  int        `json:"vert"`
	Tangent               [3]float64 `json:"tangent"`
	Sign                  float64    `json:"sign"`
	Bitangent             [3]float64 `json:"bitangent"`
	MagS                  float64    `json:"magS"`
	MagT                  float64    `json:"magT"`
	OrientationPreserving bool       `json:"orientationPreserving"`
}

type resultEntry struct {
	tangent               [3]float32
	sign                  float32
	bitangent             [3]float32
	magS, magT            float32
	orientationPreserving bool
}

func TestGoldenComparison(t *testing.T) {
	// Load GLB
	mesh, err := loadGLB("testdata/NormalTangentTest.glb")
	if err != nil {
		t.Fatalf("failed to load GLB: %v", err)
	}

	numIndices := len(mesh.indices)
	numFaces := numIndices / 3
	t.Logf("Loaded mesh: %d vertices, %d faces, %d indices",
		len(mesh.positions)/3, numFaces, numIndices)

	// Load golden data
	goldenData, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatalf("failed to load golden.json: %v", err)
	}
	var golden []goldenEntry
	if err := json.Unmarshal(goldenData, &golden); err != nil {
		t.Fatalf("failed to parse golden.json: %v", err)
	}
	t.Logf("Loaded %d golden entries", len(golden))

	expectedEntries := numFaces * 3
	if len(golden) != expectedEntries {
		t.Fatalf("golden entry count mismatch: got %d, want %d", len(golden), expectedEntries)
	}

	// Allocate result storage
	results := make([]resultEntry, expectedEntries)

	// Callbacks
	getNumFaces := func() int { return numFaces }
	getNumVerticesOfFace := func(face int) int { return 3 }
	getPosition := func(face, vert int) [3]float32 {
		idx := mesh.indices[face*3+vert]
		return [3]float32{mesh.positions[idx*3], mesh.positions[idx*3+1], mesh.positions[idx*3+2]}
	}
	getNormal := func(face, vert int) [3]float32 {
		idx := mesh.indices[face*3+vert]
		return [3]float32{mesh.normals[idx*3], mesh.normals[idx*3+1], mesh.normals[idx*3+2]}
	}
	getTexCoord := func(face, vert int) [2]float32 {
		idx := mesh.indices[face*3+vert]
		return [2]float32{mesh.texcoords[idx*2], mesh.texcoords[idx*2+1]}
	}

	// setTSpace is called first per the C implementation ordering
	setTSpace := func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool) {
		i := face*3 + vert
		results[i].tangent = tangent
		results[i].bitangent = bitangent
		results[i].magS = magS
		results[i].magT = magT
		results[i].orientationPreserving = isOrientationPreserving
	}

	// setTSpaceBasic is called after setTSpace
	setTSpaceBasic := func(face, vert int, tangent [3]float32, sign float32) {
		i := face*3 + vert
		results[i].tangent = tangent
		results[i].sign = sign
	}

	ok := GenTangSpaceDefault(
		getNumFaces,
		getNumVerticesOfFace,
		getPosition,
		getNormal,
		getTexCoord,
		setTSpaceBasic,
		setTSpace,
	)
	if !ok {
		t.Fatal("GenTangSpaceDefault returned false")
	}

	// Compare results
	const eps = 1e-5
	mismatches := 0
	maxMismatches := 20

	approxEq := func(a float32, b float64) bool {
		return math.Abs(float64(a)-b) < eps
	}

	for i, g := range golden {
		r := results[i]
		face := g.Face
		vert := g.Vert

		// Verify face/vert index
		expectedFace := i / 3
		expectedVert := i % 3
		if face != expectedFace || vert != expectedVert {
			t.Errorf("entry %d: expected face=%d vert=%d, golden has face=%d vert=%d",
				i, expectedFace, expectedVert, face, vert)
		}

		mismatch := false
		var details string

		// Check tangent
		for c := 0; c < 3; c++ {
			if !approxEq(r.tangent[c], g.Tangent[c]) {
				mismatch = true
				details += fmt.Sprintf("  tangent[%d]: got %v, want %v (diff=%e)\n",
					c, r.tangent[c], g.Tangent[c], float64(r.tangent[c])-g.Tangent[c])
			}
		}

		// Check sign
		if !approxEq(r.sign, g.Sign) {
			mismatch = true
			details += fmt.Sprintf("  sign: got %v, want %v\n", r.sign, g.Sign)
		}

		// Check bitangent
		for c := 0; c < 3; c++ {
			if !approxEq(r.bitangent[c], g.Bitangent[c]) {
				mismatch = true
				details += fmt.Sprintf("  bitangent[%d]: got %v, want %v (diff=%e)\n",
					c, r.bitangent[c], g.Bitangent[c], float64(r.bitangent[c])-g.Bitangent[c])
			}
		}

		// Check magS/magT
		if !approxEq(r.magS, g.MagS) {
			mismatch = true
			details += fmt.Sprintf("  magS: got %v, want %v (diff=%e)\n", r.magS, g.MagS, float64(r.magS)-g.MagS)
		}
		if !approxEq(r.magT, g.MagT) {
			mismatch = true
			details += fmt.Sprintf("  magT: got %v, want %v (diff=%e)\n", r.magT, g.MagT, float64(r.magT)-g.MagT)
		}

		// Check orientation
		if r.orientationPreserving != g.OrientationPreserving {
			mismatch = true
			details += fmt.Sprintf("  orientationPreserving: got %v, want %v\n",
				r.orientationPreserving, g.OrientationPreserving)
		}

		if mismatch {
			mismatches++
			if mismatches <= maxMismatches {
				t.Errorf("mismatch at face=%d vert=%d (entry %d):\n%s", face, vert, i, details)
			}
			if mismatches == maxMismatches {
				t.Errorf("... stopping after %d mismatches", maxMismatches)
				break
			}
		}
	}

	if mismatches == 0 {
		t.Logf("All %d entries match golden reference (epsilon=%e)", len(golden), eps)
	} else {
		t.Errorf("Total mismatches: %d / %d", mismatches, len(golden))
	}
}
