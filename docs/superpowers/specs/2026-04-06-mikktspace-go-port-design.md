# MikkTSpace Go Port Design

## Overview

A faithful 1:1 port of the MikkTSpace C library (~1900 lines) to Go. MikkTSpace computes tangent space for normal-mapped meshes and is the industry standard used by Blender, Filament, Unity, Unreal, and others.

The Go port preserves the exact algorithm, variable names, control flow, and numerical behavior of the C original to enable line-by-line verification and produce bit-comparable output.

## Source

- C source: `../MikkTSpace/mikktspace.h` and `../MikkTSpace/mikktspace.c`
- License: MIT-like (open source, no warranty)

## Public API

Two exported functions with function-argument callbacks:

```go
package mikktspace

func GenTangSpace(
    getNumFaces          func() int,
    getNumVerticesOfFace func(face int) int,
    getPosition          func(face, vert int) [3]float32,
    getNormal            func(face, vert int) [3]float32,
    getTexCoord          func(face, vert int) [2]float32,
    setTSpaceBasic       func(face, vert int, tangent [3]float32, sign float32),
    setTSpace            func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool),
    angularThreshold     float32,
) bool

func GenTangSpaceDefault(
    getNumFaces          func() int,
    getNumVerticesOfFace func(face int) int,
    getPosition          func(face, vert int) [3]float32,
    getNormal            func(face, vert int) [3]float32,
    getTexCoord          func(face, vert int) [2]float32,
    setTSpaceBasic       func(face, vert int, tangent [3]float32, sign float32),
    setTSpace            func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool),
) bool
```

### Callback semantics

- **Input callbacks** (`getNumFaces`, `getNumVerticesOfFace`, `getPosition`, `getNormal`, `getTexCoord`): All required, must not be nil.
- **Output callbacks** (`setTSpaceBasic`, `setTSpace`): At least one must be non-nil. Either can be nil independently.
- `getNumVerticesOfFace` must return 3 (triangle) or 4 (quad).
- `angularThreshold`: 0.0 to 180.0 degrees. `GenTangSpaceDefault` uses 180.0 (smoothing disabled).

### Design rationale

Function arguments instead of an interface type because:
- Matches the C function-pointer design directly
- Callers close over their data with no wrapper struct needed
- Simpler than defining and implementing a Go interface

## File Structure

Single source file mirroring the C single-file structure:

```
mikktspace.go          # All exported + internal code
mikktspace_test.go     # Golden output test
testdata/
  NormalTangentTest.glb   # glTF test model (no tangent attribute)
  golden.json             # Reference output from C implementation
cmd/
  gendump/
    main.c              # C reference dumper (cgltf + mikktspace)
    Makefile
```

## Internal Types

Direct 1:1 translations of C structs, unexported:

```go
type sVec3 struct{ x, y, z float32 }

type sTriInfo struct {
    faceNeighbors  [3]int
    assignedGroup  [3]*sGroup
    vOs, vOt       sVec3
    fMagS, fMagT   float32
    iOrgFaceNumber int
    iFlag          int
    iTSpacesOffs   int
    vertNum        [4]uint8
}

type sGroup struct {
    iNrFaces              int
    pFaceIndices          []int
    iVertexRepresentitive int
    bOrientPreservering   bool
}

type sTSpace struct {
    vOs, vOt     sVec3
    fMagS, fMagT float32
    iCounter     int
    bOrient      bool
}

type sSubGroup struct {
    iNrFaces    int
    pTriMembers []int
}
```

### Flag constants

```go
const (
    markDegenerate  = 1
    quadOneDegenTri = 2
    groupWithAny    = 4
)
```

## Internal Functions

Mechanical translation preserving C function names (unexported). Key functions:

### Algorithm phases

1. **Initialization & vertex welding**: `generateSharedVerticesIndexList` (hash-based, grid size 2048), `generateSharedVerticesIndexListSlow` (O(n^2) fallback)
2. **Degenerate marking**: `degenPrologue` — mark degenerate triangles, reorder to end
3. **Triangle evaluation**: `initTriInfo` — compute texture-space derivatives (equations 18-19 from Mikkelsen's thesis), build neighbor list via `buildNeighborsFast` (quicksort-based) or `buildNeighborsSlow` (O(n^2) fallback)
4. **Group formation**: `build4RuleGroups` — 4-rule connectivity algorithm (adjacency, same face, orientation, angular threshold)
5. **Tangent generation**: `generateTSpaces` — angle-weighted averaging per group, output via callbacks
6. **Degenerate resolution**: `degenEpilogue` — copy tangent space from nearest good triangle

### Quicksort

Hand-rolled 3-way partition quicksort with hardcoded seed `39871946`, not stdlib sort. This is required for deterministic output matching the C implementation.

### Vector math

`vadd`, `vsub`, `vscale`, `vdot`, `lengthSquared`, `length`, `normalize`, `notZero`, `vNotZero` — all unexported, matching C implementations.

## Translation Rules

These rules govern the mechanical C-to-Go translation:

| C construct | Go equivalent |
|-------------|---------------|
| `malloc` arrays | Slice allocation (`make([]T, n)`) |
| `tbool` / `TTRUE` / `TFALSE` | `bool` / `true` / `false` |
| `float` | `float32` |
| Pointer arithmetic | Slice indexing |
| `NULL` | `nil` |
| `assert` | `panic` (or skip — C asserts are debug-only) |
| `free` | Not needed (GC) |
| C function pointers | Go function values |
| `SMikkTSpaceContext.m_pUserData` | Not needed (closures capture data) |
| `SMikkTSpaceContext.m_pInterface` | Individual function arguments |

## Verification

### C reference dumper

A one-time tool (not part of the Go module, not shipped). A small C program (`cmd/gendump/`) that:
1. Uses cgltf (header-only C glTF loader) to load `NormalTangentTest.glb`
2. Implements MikkTSpace callbacks over the loaded mesh data
3. Runs `genTangSpaceDefault`
4. Captures every `setTSpaceBasic` and `setTSpace` callback invocation
5. Writes output to `golden.json` as an array of entries:

```json
[
  {
    "face": 0,
    "vert": 0,
    "tangent": [0.123, 0.456, 0.789],
    "sign": 1.0,
    "bitangent": [0.111, 0.222, 0.333],
    "magS": 1.0,
    "magT": 1.0,
    "orientationPreserving": true
  }
]
```

### Go test

`mikktspace_test.go`:
1. Contains a minimal glTF reader — just enough to extract positions, normals, UVs, and indices from `NormalTangentTest.glb`. No external dependencies.
2. Feeds the mesh data into `GenTangSpaceDefault` via closures
3. Collects all output callback invocations
4. Loads `golden.json` and compares every entry
5. Uses epsilon tolerance ~1e-5 for float comparison

### Test model

NormalTangentTest from glTF-Sample-Assets:
- 3,983 vertices, ~23K indices
- No TANGENT attribute (designed to test tangent generation)
- Copied to `testdata/NormalTangentTest.glb`

### What this proves

Given identical input, the Go port produces the same tangent output as the C original for a real-world model. The golden file serves as a permanent regression test.

## Scope Exclusions

- No idiomatic Go refactoring (mechanical translation only)
- No cgo — pure Go
- No external dependencies in the library itself
- No rendering or visual verification (numerical comparison only)
- No performance optimization beyond what the C code does
