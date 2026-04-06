# MikkTSpace Go Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the MikkTSpace C library (~1900 lines) to Go as a faithful 1:1 translation.

**Architecture:** Single Go file (`mikktspace.go`) with all types, constants, vector math, and algorithm functions. Public API is two exported functions with function-argument callbacks. Verification via golden output comparison against the C reference implementation.

**Tech Stack:** Go 1.25, pure stdlib (math package only). C reference dumper uses cgltf + mikktspace.c.

**Spec:** `docs/superpowers/specs/2026-04-06-mikktspace-go-port-design.md`

**C Source Reference:** `../MikkTSpace/mikktspace.c` (1899 lines)

---

### Task 1: Internal types, constants, and vector math

**Files:**
- Create: `mikktspace.go`

This task creates the foundation: package declaration, all internal types, flag constants, and vector math utilities. These correspond to C lines 33-170.

- [ ] **Step 1: Create mikktspace.go with types and vector math**

```go
package mikktspace

import "math"

// flag constants (C lines 134-137)
const (
	markDegenerate  = 1
	quadOneDegenTri = 2
	groupWithAny    = 4
	orientPreserving = 8
)

const internalRndSortSeed = 39871946
const gICells = 2048

// internal types (C lines 43-162)

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
	vOs  sVec3
	fMagS float32
	vOt  sVec3
	fMagT float32
	iCounter int
	bOrient  bool
}

type sSubGroup struct {
	iNrFaces    int
	pTriMembers []int
}

type sEdge struct {
	i0, i1, f int
}

type sTmpVert struct {
	vert  [3]float32
	index int
}

// vector math (C lines 47-117)

func veq(v1, v2 sVec3) bool {
	return v1.x == v2.x && v1.y == v2.y && v1.z == v2.z
}

func vadd(v1, v2 sVec3) sVec3 {
	return sVec3{v1.x + v2.x, v1.y + v2.y, v1.z + v2.z}
}

func vsub(v1, v2 sVec3) sVec3 {
	return sVec3{v1.x - v2.x, v1.y - v2.y, v1.z - v2.z}
}

func vscale(fS float32, v sVec3) sVec3 {
	return sVec3{fS * v.x, fS * v.y, fS * v.z}
}

func lengthSquared(v sVec3) float32 {
	return v.x*v.x + v.y*v.y + v.z*v.z
}

func vlength(v sVec3) float32 {
	return float32(math.Sqrt(float64(lengthSquared(v))))
}

func vnormalize(v sVec3) sVec3 {
	return vscale(1.0/vlength(v), v)
}

func vdot(v1, v2 sVec3) float32 {
	return v1.x*v2.x + v1.y*v2.y + v1.z*v2.z
}

// C FLT_MIN = smallest normal float32 = 1.1754944e-38
const fltMin = float32(1.1754944e-38)

func notZero(fX float32) bool {
	if fX < 0 {
		fX = -fX
	}
	return fX > fltMin
}

func vNotZero(v sVec3) bool {
	return notZero(v.x) || notZero(v.y) || notZero(v.z)
}

// index helpers (C lines 172-182)

func makeIndex(iFace, iVert int) int {
	return (iFace << 2) | (iVert & 0x3)
}

func indexToData(iIndexIn int) (iFace, iVert int) {
	return iIndexIn >> 2, iIndexIn & 0x3
}

// avgTSpace (C lines 184-210)

func avgTSpace(pTS0, pTS1 *sTSpace) sTSpace {
	if pTS0.fMagS == pTS1.fMagS && pTS0.fMagT == pTS1.fMagT &&
		veq(pTS0.vOs, pTS1.vOs) && veq(pTS0.vOt, pTS1.vOt) {
		return sTSpace{
			vOs: pTS0.vOs, fMagS: pTS0.fMagS,
			vOt: pTS0.vOt, fMagT: pTS0.fMagT,
		}
	}
	tsRes := sTSpace{
		fMagS: 0.5 * (pTS0.fMagS + pTS1.fMagS),
		fMagT: 0.5 * (pTS0.fMagT + pTS1.fMagT),
		vOs:   vadd(pTS0.vOs, pTS1.vOs),
		vOt:   vadd(pTS0.vOt, pTS1.vOt),
	}
	if vNotZero(tsRes.vOs) {
		tsRes.vOs = vnormalize(tsRes.vOs)
	}
	if vNotZero(tsRes.vOt) {
		tsRes.vOt = vnormalize(tsRes.vOt)
	}
	return tsRes
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success (no errors)

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add internal types, constants, and vector math for MikkTSpace port"
```

---

### Task 2: Public API, getPosition/getNormal/getTexCoord, and main function body

**Files:**
- Modify: `mikktspace.go`

Add the two exported functions and the main `genTangSpace` body (C lines 224-420), plus the context-wrapping helper functions `getPosition`, `getNormal`, `getTexCoord` (C lines 883-911). Since Go uses closures instead of context structs, internal functions receive the callbacks as a bundled struct.

- [ ] **Step 1: Add internal context struct and getter helpers**

Append to `mikktspace.go`:

```go
// internal context bundles all callbacks to avoid passing 7 function args everywhere
type context struct {
	getNumFaces          func() int
	getNumVerticesOfFace func(face int) int
	getPosition          func(face, vert int) [3]float32
	getNormal            func(face, vert int) [3]float32
	getTexCoord          func(face, vert int) [2]float32
	setTSpaceBasic       func(face, vert int, tangent [3]float32, sign float32)
	setTSpace            func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool)
}

// C lines 883-911: get vertex data by packed index
func getPosition(ctx *context, index int) sVec3 {
	iFace, iVert := indexToData(index)
	pos := ctx.getPosition(iFace, iVert)
	return sVec3{pos[0], pos[1], pos[2]}
}

func getNormal(ctx *context, index int) sVec3 {
	iFace, iVert := indexToData(index)
	norm := ctx.getNormal(iFace, iVert)
	return sVec3{norm[0], norm[1], norm[2]}
}

func getTexCoord(ctx *context, index int) sVec3 {
	iFace, iVert := indexToData(index)
	texc := ctx.getTexCoord(iFace, iVert)
	return sVec3{texc[0], texc[1], 1.0}
}
```

- [ ] **Step 2: Add the main GenTangSpace and GenTangSpaceDefault functions**

Append to `mikktspace.go`. This is the main algorithm orchestrator (C lines 224-420):

```go
// GenTangSpaceDefault calls GenTangSpace with angular threshold = 180 (disabled).
func GenTangSpaceDefault(
	getNumFaces func() int,
	getNumVerticesOfFace func(face int) int,
	getPosition func(face, vert int) [3]float32,
	getNormal func(face, vert int) [3]float32,
	getTexCoord func(face, vert int) [2]float32,
	setTSpaceBasic func(face, vert int, tangent [3]float32, sign float32),
	setTSpace func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool),
) bool {
	return GenTangSpace(getNumFaces, getNumVerticesOfFace, getPosition, getNormal, getTexCoord, setTSpaceBasic, setTSpace, 180.0)
}

// GenTangSpace computes tangent space with a configurable angular threshold (0-180 degrees).
func GenTangSpace(
	getNumFaces func() int,
	getNumVerticesOfFace func(face int) int,
	gpPosition func(face, vert int) [3]float32,
	gpNormal func(face, vert int) [3]float32,
	gpTexCoord func(face, vert int) [2]float32,
	setTSpaceBasic func(face, vert int, tangent [3]float32, sign float32),
	setTSpace func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, isOrientationPreserving bool),
	fAngularThreshold float32,
) bool {
	ctx := &context{
		getNumFaces:          getNumFaces,
		getNumVerticesOfFace: getNumVerticesOfFace,
		getPosition:          gpPosition,
		getNormal:            gpNormal,
		getTexCoord:          gpTexCoord,
		setTSpaceBasic:       setTSpaceBasic,
		setTSpace:            setTSpace,
	}

	// verify all input callbacks have been set
	if ctx.getNumFaces == nil || ctx.getNumVerticesOfFace == nil ||
		ctx.getPosition == nil || ctx.getNormal == nil || ctx.getTexCoord == nil {
		return false
	}

	iNrFaces := ctx.getNumFaces()
	fThresCos := float32(math.Cos(float64(fAngularThreshold) * math.Pi / 180.0))

	// count triangles on supported faces
	iNrTrianglesIn := 0
	for f := 0; f < iNrFaces; f++ {
		verts := ctx.getNumVerticesOfFace(f)
		if verts == 3 {
			iNrTrianglesIn++
		} else if verts == 4 {
			iNrTrianglesIn += 2
		}
	}
	if iNrTrianglesIn <= 0 {
		return false
	}

	piTriListIn := make([]int, 3*iNrTrianglesIn)
	pTriInfos := make([]sTriInfo, iNrTrianglesIn)

	// make an initial triangle --> face index list
	iNrTSPaces := generateInitialVerticesIndexList(pTriInfos, piTriListIn, ctx, iNrTrianglesIn)

	// make a welded index list of identical positions and attributes (pos, norm, texc)
	generateSharedVerticesIndexList(piTriListIn, ctx, iNrTrianglesIn)

	// Mark all degenerate triangles
	iTotTris := iNrTrianglesIn
	iDegenTriangles := 0
	for t := 0; t < iTotTris; t++ {
		i0 := piTriListIn[t*3+0]
		i1 := piTriListIn[t*3+1]
		i2 := piTriListIn[t*3+2]
		p0 := getPosition(ctx, i0)
		p1 := getPosition(ctx, i1)
		p2 := getPosition(ctx, i2)
		if veq(p0, p1) || veq(p0, p2) || veq(p1, p2) {
			pTriInfos[t].iFlag |= markDegenerate
			iDegenTriangles++
		}
	}
	iNrTrianglesIn = iTotTris - iDegenTriangles

	// mark quad pairs with one degenerate, move good triangles to front
	degenPrologue(pTriInfos, piTriListIn, iNrTrianglesIn, iTotTris)

	// evaluate triangle level attributes and neighbor list
	initTriInfo(pTriInfos, piTriListIn, ctx, iNrTrianglesIn)

	// based on the 4 rules, identify groups based on connectivity
	iNrMaxGroups := iNrTrianglesIn * 3
	pGroups := make([]sGroup, iNrMaxGroups)
	piGroupTrianglesBuffer := make([]int, iNrTrianglesIn*3)

	iNrActiveGroups := build4RuleGroups(pTriInfos, pGroups, piGroupTrianglesBuffer, piTriListIn, iNrTrianglesIn)

	psTspace := make([]sTSpace, iNrTSPaces)
	for t := 0; t < iNrTSPaces; t++ {
		psTspace[t].vOs = sVec3{1.0, 0.0, 0.0}
		psTspace[t].fMagS = 1.0
		psTspace[t].vOt = sVec3{0.0, 1.0, 0.0}
		psTspace[t].fMagT = 1.0
	}

	// make tspaces
	bRes := generateTSpaces(psTspace, pTriInfos, pGroups, iNrActiveGroups, piTriListIn, fThresCos, ctx)

	if !bRes {
		return false
	}

	// handle degenerate triangles
	degenEpilogue(psTspace, pTriInfos, piTriListIn, ctx, iNrTrianglesIn, iTotTris)

	// output results
	index := 0
	for f := 0; f < iNrFaces; f++ {
		verts := ctx.getNumVerticesOfFace(f)
		if verts != 3 && verts != 4 {
			continue
		}

		for i := 0; i < verts; i++ {
			pTSpace := &psTspace[index]
			tang := [3]float32{pTSpace.vOs.x, pTSpace.vOs.y, pTSpace.vOs.z}
			bitang := [3]float32{pTSpace.vOt.x, pTSpace.vOt.y, pTSpace.vOt.z}
			if ctx.setTSpace != nil {
				ctx.setTSpace(f, i, tang, bitang, pTSpace.fMagS, pTSpace.fMagT, pTSpace.bOrient)
			}
			if ctx.setTSpaceBasic != nil {
				var fSign float32
				if pTSpace.bOrient {
					fSign = 1.0
				} else {
					fSign = -1.0
				}
				ctx.setTSpaceBasic(f, i, tang, fSign)
			}
			index++
		}
	}

	return true
}
```

- [ ] **Step 3: Add stub functions so it compiles**

Add empty stubs that will be implemented in subsequent tasks:

```go
func generateInitialVerticesIndexList(pTriInfos []sTriInfo, piTriListOut []int, ctx *context, iNrTrianglesIn int) int {
	panic("not yet implemented")
}

func generateSharedVerticesIndexList(piTriListInAndOut []int, ctx *context, iNrTrianglesIn int) {
	panic("not yet implemented")
}

func initTriInfo(pTriInfos []sTriInfo, piTriListIn []int, ctx *context, iNrTrianglesIn int) {
	panic("not yet implemented")
}

func build4RuleGroups(pTriInfos []sTriInfo, pGroups []sGroup, piGroupTrianglesBuffer []int, piTriListIn []int, iNrTrianglesIn int) int {
	panic("not yet implemented")
}

func generateTSpaces(psTspace []sTSpace, pTriInfos []sTriInfo, pGroups []sGroup, iNrActiveGroups int, piTriListIn []int, fThresCos float32, ctx *context) bool {
	panic("not yet implemented")
}

func degenPrologue(pTriInfos []sTriInfo, piTriListOut []int, iNrTrianglesIn int, iTotTris int) {
	panic("not yet implemented")
}

func degenEpilogue(psTspace []sTSpace, pTriInfos []sTriInfo, piTriListIn []int, ctx *context, iNrTrianglesIn int, iTotTris int) {
	panic("not yet implemented")
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add public API and main genTangSpace function body"
```

---

### Task 3: generateInitialVerticesIndexList

**Files:**
- Modify: `mikktspace.go`

Translate C lines 772-881. This function initializes the triangle list from faces, handling quad triangulation by shortest diagonal.

- [ ] **Step 1: Replace the stub with the full implementation**

Replace the `generateInitialVerticesIndexList` stub with:

```go
// C lines 772-881
func generateInitialVerticesIndexList(pTriInfos []sTriInfo, piTriListOut []int, ctx *context, iNrTrianglesIn int) int {
	iTSpacesOffs := 0
	iDstTriIndex := 0
	iNrFaces := ctx.getNumFaces()

	for f := 0; f < iNrFaces; f++ {
		verts := ctx.getNumVerticesOfFace(f)
		if verts != 3 && verts != 4 {
			continue
		}

		pTriInfos[iDstTriIndex].iOrgFaceNumber = f
		pTriInfos[iDstTriIndex].iTSpacesOffs = iTSpacesOffs

		if verts == 3 {
			pVerts := &pTriInfos[iDstTriIndex].vertNum
			pVerts[0] = 0
			pVerts[1] = 1
			pVerts[2] = 2
			piTriListOut[iDstTriIndex*3+0] = makeIndex(f, 0)
			piTriListOut[iDstTriIndex*3+1] = makeIndex(f, 1)
			piTriListOut[iDstTriIndex*3+2] = makeIndex(f, 2)
			iDstTriIndex++
		} else {
			pTriInfos[iDstTriIndex+1].iOrgFaceNumber = f
			pTriInfos[iDstTriIndex+1].iTSpacesOffs = iTSpacesOffs

			i0 := makeIndex(f, 0)
			i1 := makeIndex(f, 1)
			i2 := makeIndex(f, 2)
			i3 := makeIndex(f, 3)
			T0 := getTexCoord(ctx, i0)
			T1 := getTexCoord(ctx, i1)
			T2 := getTexCoord(ctx, i2)
			T3 := getTexCoord(ctx, i3)
			distSQ_02 := lengthSquared(vsub(T2, T0))
			distSQ_13 := lengthSquared(vsub(T3, T1))

			var bQuadDiagIs_02 bool
			if distSQ_02 < distSQ_13 {
				bQuadDiagIs_02 = true
			} else if distSQ_13 < distSQ_02 {
				bQuadDiagIs_02 = false
			} else {
				P0 := getPosition(ctx, i0)
				P1 := getPosition(ctx, i1)
				P2 := getPosition(ctx, i2)
				P3 := getPosition(ctx, i3)
				distSQ_02p := lengthSquared(vsub(P2, P0))
				distSQ_13p := lengthSquared(vsub(P3, P1))
				bQuadDiagIs_02 = !(distSQ_13p < distSQ_02p)
			}

			if bQuadDiagIs_02 {
				pVertsA := &pTriInfos[iDstTriIndex].vertNum
				pVertsA[0] = 0
				pVertsA[1] = 1
				pVertsA[2] = 2
				piTriListOut[iDstTriIndex*3+0] = i0
				piTriListOut[iDstTriIndex*3+1] = i1
				piTriListOut[iDstTriIndex*3+2] = i2
				iDstTriIndex++

				pVertsB := &pTriInfos[iDstTriIndex].vertNum
				pVertsB[0] = 0
				pVertsB[1] = 2
				pVertsB[2] = 3
				piTriListOut[iDstTriIndex*3+0] = i0
				piTriListOut[iDstTriIndex*3+1] = i2
				piTriListOut[iDstTriIndex*3+2] = i3
				iDstTriIndex++
			} else {
				pVertsA := &pTriInfos[iDstTriIndex].vertNum
				pVertsA[0] = 0
				pVertsA[1] = 1
				pVertsA[2] = 3
				piTriListOut[iDstTriIndex*3+0] = i0
				piTriListOut[iDstTriIndex*3+1] = i1
				piTriListOut[iDstTriIndex*3+2] = i3
				iDstTriIndex++

				pVertsB := &pTriInfos[iDstTriIndex].vertNum
				pVertsB[0] = 1
				pVertsB[1] = 2
				pVertsB[2] = 3
				piTriListOut[iDstTriIndex*3+0] = i1
				piTriListOut[iDstTriIndex*3+1] = i2
				piTriListOut[iDstTriIndex*3+2] = i3
				iDstTriIndex++
			}
		}

		iTSpacesOffs += verts
	}

	for t := 0; t < iNrTrianglesIn; t++ {
		pTriInfos[t].iFlag = 0
	}

	return iTSpacesOffs
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add generateInitialVerticesIndexList"
```

---

### Task 4: Vertex welding (shared vertices index list)

**Files:**
- Modify: `mikktspace.go`

Translate C lines 429-770: `findGridCell`, `generateSharedVerticesIndexList`, `mergeVertsFast`, `mergeVertsSlow`, `generateSharedVerticesIndexListSlow`.

- [ ] **Step 1: Replace the generateSharedVerticesIndexList stub and add all welding functions**

Replace the `generateSharedVerticesIndexList` stub and add the supporting functions:

```go
// C line 440-445
func findGridCell(fMin, fMax, fVal float32) int {
	fIndex := float32(gICells) * ((fVal - fMin) / (fMax - fMin))
	iIndex := int(fIndex)
	if iIndex < gICells {
		if iIndex >= 0 {
			return iIndex
		}
		return 0
	}
	return gICells - 1
}

// C lines 451-576
func generateSharedVerticesIndexList(piTriListInAndOut []int, ctx *context, iNrTrianglesIn int) {
	// Generate bounding box
	vMin := getPosition(ctx, piTriListInAndOut[0])
	vMax := vMin
	for i := 1; i < iNrTrianglesIn*3; i++ {
		index := piTriListInAndOut[i]
		vP := getPosition(ctx, index)
		if vMin.x > vP.x {
			vMin.x = vP.x
		} else if vMax.x < vP.x {
			vMax.x = vP.x
		}
		if vMin.y > vP.y {
			vMin.y = vP.y
		} else if vMax.y < vP.y {
			vMax.y = vP.y
		}
		if vMin.z > vP.z {
			vMin.z = vP.z
		} else if vMax.z < vP.z {
			vMax.z = vP.z
		}
	}

	vDim := vsub(vMax, vMin)
	iChannel := 0
	fMin := vMin.x
	fMax := vMax.x
	if vDim.y > vDim.x && vDim.y > vDim.z {
		iChannel = 1
		fMin = vMin.y
		fMax = vMax.y
	} else if vDim.z > vDim.x {
		iChannel = 2
		fMin = vMin.z
		fMax = vMax.z
	}

	// make allocations
	piHashTable := make([]int, iNrTrianglesIn*3)
	piHashCount := make([]int, gICells)
	piHashOffsets := make([]int, gICells)
	piHashCount2 := make([]int, gICells)

	// count amount of elements in each cell unit
	for i := 0; i < iNrTrianglesIn*3; i++ {
		index := piTriListInAndOut[i]
		vP := getPosition(ctx, index)
		var fVal float32
		switch iChannel {
		case 0:
			fVal = vP.x
		case 1:
			fVal = vP.y
		default:
			fVal = vP.z
		}
		iCell := findGridCell(fMin, fMax, fVal)
		piHashCount[iCell]++
	}

	// evaluate start index of each cell
	piHashOffsets[0] = 0
	for k := 1; k < gICells; k++ {
		piHashOffsets[k] = piHashOffsets[k-1] + piHashCount[k-1]
	}

	// insert vertices
	for i := 0; i < iNrTrianglesIn*3; i++ {
		index := piTriListInAndOut[i]
		vP := getPosition(ctx, index)
		var fVal float32
		switch iChannel {
		case 0:
			fVal = vP.x
		case 1:
			fVal = vP.y
		default:
			fVal = vP.z
		}
		iCell := findGridCell(fMin, fMax, fVal)
		piHashTable[piHashOffsets[iCell]+piHashCount2[iCell]] = i
		piHashCount2[iCell]++
	}

	// find maximum amount of entries in any hash entry
	iMaxCount := piHashCount[0]
	for k := 1; k < gICells; k++ {
		if iMaxCount < piHashCount[k] {
			iMaxCount = piHashCount[k]
		}
	}
	pTmpVert := make([]sTmpVert, iMaxCount)

	// complete the merge
	for k := 0; k < gICells; k++ {
		pTableOffset := piHashOffsets[k]
		iEntries := piHashCount[k]
		if iEntries < 2 {
			continue
		}

		for e := 0; e < iEntries; e++ {
			i := piHashTable[pTableOffset+e]
			vP := getPosition(ctx, piTriListInAndOut[i])
			pTmpVert[e].vert[0] = vP.x
			pTmpVert[e].vert[1] = vP.y
			pTmpVert[e].vert[2] = vP.z
			pTmpVert[e].index = i
		}
		mergeVertsFast(piTriListInAndOut, pTmpVert, ctx, 0, iEntries-1)
	}
}

// C lines 578-692
func mergeVertsFast(piTriListInAndOut []int, pTmpVert []sTmpVert, ctx *context, iLIn, iRIn int) {
	// make bbox
	var fvMin, fvMax [3]float32
	for c := 0; c < 3; c++ {
		fvMin[c] = pTmpVert[iLIn].vert[c]
		fvMax[c] = fvMin[c]
	}
	for l := iLIn + 1; l <= iRIn; l++ {
		for c := 0; c < 3; c++ {
			if fvMin[c] > pTmpVert[l].vert[c] {
				fvMin[c] = pTmpVert[l].vert[c]
			}
			if fvMax[c] < pTmpVert[l].vert[c] {
				fvMax[c] = pTmpVert[l].vert[c]
			}
		}
	}

	dx := fvMax[0] - fvMin[0]
	dy := fvMax[1] - fvMin[1]
	dz := fvMax[2] - fvMin[2]

	channel := 0
	if dy > dx && dy > dz {
		channel = 1
	} else if dz > dx {
		channel = 2
	}

	fSep := 0.5 * (fvMax[channel] + fvMin[channel])

	// stop if all vertices are NaNs
	if math.IsInf(float64(fSep), 0) || math.IsNaN(float64(fSep)) {
		return
	}

	// terminate recursion when the separation/average value
	// is no longer strictly between fMin and fMax values.
	if fSep >= fvMax[channel] || fSep <= fvMin[channel] {
		// complete the weld
		for l := iLIn; l <= iRIn; l++ {
			i := pTmpVert[l].index
			index := piTriListInAndOut[i]
			vP := getPosition(ctx, index)
			vN := getNormal(ctx, index)
			vT := getTexCoord(ctx, index)

			bNotFound := true
			l2 := iLIn
			i2rec := -1
			for l2 < l && bNotFound {
				i2 := pTmpVert[l2].index
				index2 := piTriListInAndOut[i2]
				vP2 := getPosition(ctx, index2)
				vN2 := getNormal(ctx, index2)
				vT2 := getTexCoord(ctx, index2)
				i2rec = i2

				if vP.x == vP2.x && vP.y == vP2.y && vP.z == vP2.z &&
					vN.x == vN2.x && vN.y == vN2.y && vN.z == vN2.z &&
					vT.x == vT2.x && vT.y == vT2.y && vT.z == vT2.z {
					bNotFound = false
				} else {
					l2++
				}
			}

			if !bNotFound {
				piTriListInAndOut[i] = piTriListInAndOut[i2rec]
			}
		}
	} else {
		iL := iLIn
		iR := iRIn

		// separate (by fSep) all points between iLIn and iRIn in pTmpVert[]
		for iL < iR {
			bReadyLeftSwap := false
			bReadyRightSwap := false
			for !bReadyLeftSwap && iL < iR {
				bReadyLeftSwap = !(pTmpVert[iL].vert[channel] < fSep)
				if !bReadyLeftSwap {
					iL++
				}
			}
			for !bReadyRightSwap && iL < iR {
				bReadyRightSwap = pTmpVert[iR].vert[channel] < fSep
				if !bReadyRightSwap {
					iR--
				}
			}

			if bReadyLeftSwap && bReadyRightSwap {
				sTmp := pTmpVert[iL]
				pTmpVert[iL] = pTmpVert[iR]
				pTmpVert[iR] = sTmp
				iL++
				iR--
			}
		}

		if iL == iR {
			bReadyRightSwap := pTmpVert[iR].vert[channel] < fSep
			if bReadyRightSwap {
				iL++
			} else {
				iR--
			}
		}

		if iLIn < iR {
			mergeVertsFast(piTriListInAndOut, pTmpVert, ctx, iLIn, iR)
		}
		if iL < iRIn {
			mergeVertsFast(piTriListInAndOut, pTmpVert, ctx, iL, iRIn)
		}
	}
}

// C lines 694-727
func mergeVertsSlow(piTriListInAndOut []int, ctx *context, pTable []int, iEntries int) {
	for e := 0; e < iEntries; e++ {
		i := pTable[e]
		index := piTriListInAndOut[i]
		vP := getPosition(ctx, index)
		vN := getNormal(ctx, index)
		vT := getTexCoord(ctx, index)

		bNotFound := true
		e2 := 0
		i2rec := -1
		for e2 < e && bNotFound {
			i2 := pTable[e2]
			index2 := piTriListInAndOut[i2]
			vP2 := getPosition(ctx, index2)
			vN2 := getNormal(ctx, index2)
			vT2 := getTexCoord(ctx, index2)
			i2rec = i2

			if veq(vP, vP2) && veq(vN, vN2) && veq(vT, vT2) {
				bNotFound = false
			} else {
				e2++
			}
		}

		if !bNotFound {
			piTriListInAndOut[i] = piTriListInAndOut[i2rec]
		}
	}
}

// C lines 729-770
func generateSharedVerticesIndexListSlow(piTriListInAndOut []int, ctx *context, iNrTrianglesIn int) {
	for t := 0; t < iNrTrianglesIn; t++ {
		for i := 0; i < 3; i++ {
			offs := t*3 + i
			index := piTriListInAndOut[offs]

			vP := getPosition(ctx, index)
			vN := getNormal(ctx, index)
			vT := getTexCoord(ctx, index)

			bFound := false
			t2 := 0
			index2rec := -1
			for !bFound && t2 <= t {
				j := 0
				for !bFound && j < 3 {
					index2 := piTriListInAndOut[t2*3+j]
					vP2 := getPosition(ctx, index2)
					vN2 := getNormal(ctx, index2)
					vT2 := getTexCoord(ctx, index2)

					if veq(vP, vP2) && veq(vN, vN2) && veq(vT, vT2) {
						bFound = true
						index2rec = index2
					} else {
						j++
					}
				}
				if !bFound {
					t2++
				}
			}

			piTriListInAndOut[offs] = index2rec
		}
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add vertex welding (shared vertices index list)"
```

---

### Task 5: initTriInfo, calcTexArea, and neighbor building

**Files:**
- Modify: `mikktspace.go`

Translate C lines 916-1061 (initTriInfo, calcTexArea, edge type) and C lines 1501-1731 (buildNeighborsFast, buildNeighborsSlow, quickSortEdges, getEdge).

- [ ] **Step 1: Replace the initTriInfo stub and add all supporting functions**

Replace the `initTriInfo` stub and add:

```go
// C lines 928-942
func calcTexArea(ctx *context, indices []int) float32 {
	t1 := getTexCoord(ctx, indices[0])
	t2 := getTexCoord(ctx, indices[1])
	t3 := getTexCoord(ctx, indices[2])

	t21x := t2.x - t1.x
	t21y := t2.y - t1.y
	t31x := t3.x - t1.x
	t31y := t3.y - t1.y

	fSignedAreaSTx2 := t21x*t31y - t21y*t31x

	if fSignedAreaSTx2 < 0 {
		return -fSignedAreaSTx2
	}
	return fSignedAreaSTx2
}

// C lines 944-1061
func initTriInfo(pTriInfos []sTriInfo, piTriListIn []int, ctx *context, iNrTrianglesIn int) {
	// generate neighbor info list
	for f := 0; f < iNrTrianglesIn; f++ {
		for i := 0; i < 3; i++ {
			pTriInfos[f].faceNeighbors[i] = -1
			pTriInfos[f].assignedGroup[i] = nil

			pTriInfos[f].vOs = sVec3{0, 0, 0}
			pTriInfos[f].vOt = sVec3{0, 0, 0}
			pTriInfos[f].fMagS = 0
			pTriInfos[f].fMagT = 0

			// assumed bad
			pTriInfos[f].iFlag |= groupWithAny
		}
	}

	// evaluate first order derivatives
	for f := 0; f < iNrTrianglesIn; f++ {
		v1 := getPosition(ctx, piTriListIn[f*3+0])
		v2 := getPosition(ctx, piTriListIn[f*3+1])
		v3 := getPosition(ctx, piTriListIn[f*3+2])
		t1 := getTexCoord(ctx, piTriListIn[f*3+0])
		t2 := getTexCoord(ctx, piTriListIn[f*3+1])
		t3 := getTexCoord(ctx, piTriListIn[f*3+2])

		t21x := t2.x - t1.x
		t21y := t2.y - t1.y
		t31x := t3.x - t1.x
		t31y := t3.y - t1.y
		d1 := vsub(v2, v1)
		d2 := vsub(v3, v1)

		fSignedAreaSTx2 := t21x*t31y - t21y*t31x

		vOs := vsub(vscale(t31y, d1), vscale(t21y, d2))
		vOt := vadd(vscale(-t31x, d1), vscale(t21x, d2))

		if fSignedAreaSTx2 > 0 {
			pTriInfos[f].iFlag |= orientPreserving
		}

		if notZero(fSignedAreaSTx2) {
			fAbsArea := float32(math.Abs(float64(fSignedAreaSTx2)))
			fLenOs := vlength(vOs)
			fLenOt := vlength(vOt)
			var fS float32
			if (pTriInfos[f].iFlag & orientPreserving) == 0 {
				fS = -1.0
			} else {
				fS = 1.0
			}
			if notZero(fLenOs) {
				pTriInfos[f].vOs = vscale(fS/fLenOs, vOs)
			}
			if notZero(fLenOt) {
				pTriInfos[f].vOt = vscale(fS/fLenOt, vOt)
			}

			pTriInfos[f].fMagS = fLenOs / fAbsArea
			pTriInfos[f].fMagT = fLenOt / fAbsArea

			if notZero(pTriInfos[f].fMagS) && notZero(pTriInfos[f].fMagT) {
				pTriInfos[f].iFlag &= ^groupWithAny
			}
		}
	}

	// force otherwise healthy quads to a fixed orientation
	t := 0
	for t < (iNrTrianglesIn - 1) {
		iFO_a := pTriInfos[t].iOrgFaceNumber
		iFO_b := pTriInfos[t+1].iOrgFaceNumber
		if iFO_a == iFO_b { // this is a quad
			bIsDeg_a := (pTriInfos[t].iFlag & markDegenerate) != 0
			bIsDeg_b := (pTriInfos[t+1].iFlag & markDegenerate) != 0

			if !bIsDeg_a && !bIsDeg_b {
				bOrientA := (pTriInfos[t].iFlag & orientPreserving) != 0
				bOrientB := (pTriInfos[t+1].iFlag & orientPreserving) != 0
				if bOrientA != bOrientB {
					bChooseOrientFirstTri := false
					if (pTriInfos[t+1].iFlag & groupWithAny) != 0 {
						bChooseOrientFirstTri = true
					} else if calcTexArea(ctx, piTriListIn[t*3:t*3+3]) >= calcTexArea(ctx, piTriListIn[(t+1)*3:(t+1)*3+3]) {
						bChooseOrientFirstTri = true
					}

					var t0, t1 int
					if bChooseOrientFirstTri {
						t0 = t
						t1 = t + 1
					} else {
						t0 = t + 1
						t1 = t
					}
					pTriInfos[t1].iFlag &= ^orientPreserving // clear first
					pTriInfos[t1].iFlag |= (pTriInfos[t0].iFlag & orientPreserving) // copy bit
				}
			}
			t += 2
		} else {
			t++
		}
	}

	// match up edge pairs
	pEdges := make([]sEdge, iNrTrianglesIn*3)
	buildNeighborsFast(pTriInfos, pEdges, piTriListIn, iNrTrianglesIn)
}

// C lines 1501-1594
func buildNeighborsFast(pTriInfos []sTriInfo, pEdges []sEdge, piTriListIn []int, iNrTrianglesIn int) {
	uSeed := uint32(internalRndSortSeed)

	for f := 0; f < iNrTrianglesIn; f++ {
		for i := 0; i < 3; i++ {
			i0 := piTriListIn[f*3+i]
			var i1 int
			if i < 2 {
				i1 = piTriListIn[f*3+(i+1)]
			} else {
				i1 = piTriListIn[f*3+0]
			}
			if i0 < i1 {
				pEdges[f*3+i].i0 = i0
				pEdges[f*3+i].i1 = i1
			} else {
				pEdges[f*3+i].i0 = i1
				pEdges[f*3+i].i1 = i0
			}
			pEdges[f*3+i].f = f
		}
	}

	// sort over all edges by i0
	quickSortEdges(pEdges, 0, iNrTrianglesIn*3-1, 0, uSeed)

	// sub sort over i1
	iEntries := iNrTrianglesIn * 3
	iCurStartIndex := 0
	for i := 1; i < iEntries; i++ {
		if pEdges[iCurStartIndex].i0 != pEdges[i].i0 {
			iL := iCurStartIndex
			iR := i - 1
			iCurStartIndex = i
			quickSortEdges(pEdges, iL, iR, 1, uSeed)
		}
	}

	// sub sort over f
	iCurStartIndex = 0
	for i := 1; i < iEntries; i++ {
		if pEdges[iCurStartIndex].i0 != pEdges[i].i0 || pEdges[iCurStartIndex].i1 != pEdges[i].i1 {
			iL := iCurStartIndex
			iR := i - 1
			iCurStartIndex = i
			quickSortEdges(pEdges, iL, iR, 2, uSeed)
		}
	}

	// pair up adjacent triangles
	for i := 0; i < iEntries; i++ {
		i0 := pEdges[i].i0
		i1 := pEdges[i].i1
		f := pEdges[i].f

		i0A, i1A, edgenumA := getEdge(piTriListIn[f*3:f*3+3], i0, i1)
		bUnassignedA := pTriInfos[f].faceNeighbors[edgenumA] == -1

		if bUnassignedA {
			j := i + 1
			bNotFound := true
			var edgenumB int
			for j < iEntries && i0 == pEdges[j].i0 && i1 == pEdges[j].i1 && bNotFound {
				t := pEdges[j].f
				// flip i0_B and i1_B
				i1B, i0B, edgenumB_ := getEdge(piTriListIn[t*3:t*3+3], pEdges[j].i0, pEdges[j].i1)
				edgenumB = edgenumB_
				bUnassignedB := pTriInfos[t].faceNeighbors[edgenumB] == -1
				if i0A == i0B && i1A == i1B && bUnassignedB {
					bNotFound = false
				} else {
					j++
				}
				_ = i1B // the flipped output
			}

			if !bNotFound {
				t := pEdges[j].f
				pTriInfos[f].faceNeighbors[edgenumA] = t
				pTriInfos[t].faceNeighbors[edgenumB] = f
			}
		}
	}
}

// C lines 1596-1643
func buildNeighborsSlow(pTriInfos []sTriInfo, piTriListIn []int, iNrTrianglesIn int) {
	for f := 0; f < iNrTrianglesIn; f++ {
		for i := 0; i < 3; i++ {
			if pTriInfos[f].faceNeighbors[i] == -1 {
				i0A := piTriListIn[f*3+i]
				var i1A int
				if i < 2 {
					i1A = piTriListIn[f*3+(i+1)]
				} else {
					i1A = piTriListIn[f*3+0]
				}

				bFound := false
				t := 0
				j := 0
				for !bFound && t < iNrTrianglesIn {
					if t != f {
						j = 0
						for !bFound && j < 3 {
							// in rev order
							i1B := piTriListIn[t*3+j]
							var i0B int
							if j < 2 {
								i0B = piTriListIn[t*3+(j+1)]
							} else {
								i0B = piTriListIn[t*3+0]
							}
							if i0A == i0B && i1A == i1B {
								bFound = true
							} else {
								j++
							}
						}
					}
					if !bFound {
						t++
					}
				}

				if bFound {
					pTriInfos[f].faceNeighbors[i] = t
					pTriInfos[t].faceNeighbors[j] = f
				}
			}
		}
	}
}

// C lines 1645-1700
func quickSortEdges(pSortBuffer []sEdge, iLeft, iRight, channel int, uSeed uint32) {
	iElems := iRight - iLeft + 1
	if iElems < 2 {
		return
	}
	if iElems == 2 {
		if edgeVal(pSortBuffer[iLeft], channel) > edgeVal(pSortBuffer[iRight], channel) {
			pSortBuffer[iLeft], pSortBuffer[iRight] = pSortBuffer[iRight], pSortBuffer[iLeft]
		}
		return
	}

	// Random
	t := uSeed & 31
	t = (uSeed << t) | (uSeed >> (32 - t))
	uSeed = uSeed + t + 3

	iL := iLeft
	iR := iRight
	n := (iR - iL) + 1
	index := int(uSeed % uint32(n))

	iMid := edgeVal(pSortBuffer[index+iL], channel)

	for {
		for edgeVal(pSortBuffer[iL], channel) < iMid {
			iL++
		}
		for edgeVal(pSortBuffer[iR], channel) > iMid {
			iR--
		}

		if iL <= iR {
			pSortBuffer[iL], pSortBuffer[iR] = pSortBuffer[iR], pSortBuffer[iL]
			iL++
			iR--
		}

		if iL > iR {
			break
		}
	}

	if iLeft < iR {
		quickSortEdges(pSortBuffer, iLeft, iR, channel, uSeed)
	}
	if iL < iRight {
		quickSortEdges(pSortBuffer, iL, iRight, channel, uSeed)
	}
}

// helper to access edge fields by channel index (replaces C union array access)
func edgeVal(e sEdge, channel int) int {
	switch channel {
	case 0:
		return e.i0
	case 1:
		return e.i1
	default:
		return e.f
	}
}

// C lines 1702-1731
func getEdge(indices []int, i0In, i1In int) (i0Out, i1Out, edgenumOut int) {
	edgenumOut = -1

	if indices[0] == i0In || indices[0] == i1In {
		if indices[1] == i0In || indices[1] == i1In {
			edgenumOut = 0
			i0Out = indices[0]
			i1Out = indices[1]
		} else {
			edgenumOut = 2
			i0Out = indices[2]
			i1Out = indices[0]
		}
	} else {
		edgenumOut = 1
		i0Out = indices[1]
		i1Out = indices[2]
	}
	return
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add initTriInfo, calcTexArea, and neighbor building"
```

---

### Task 6: Group formation (build4RuleGroups)

**Files:**
- Modify: `mikktspace.go`

Translate C lines 1066-1189: `build4RuleGroups`, `addTriToGroup`, `assignRecur`.

- [ ] **Step 1: Replace the build4RuleGroups stub and add supporting functions**

Replace the `build4RuleGroups` stub:

```go
// C lines 1069-1131
func build4RuleGroups(pTriInfos []sTriInfo, pGroups []sGroup, piGroupTrianglesBuffer []int, piTriListIn []int, iNrTrianglesIn int) int {
	iNrActiveGroups := 0
	iOffset := 0
	for f := 0; f < iNrTrianglesIn; f++ {
		for i := 0; i < 3; i++ {
			if (pTriInfos[f].iFlag&groupWithAny) == 0 && pTriInfos[f].assignedGroup[i] == nil {
				vertIndex := piTriListIn[f*3+i]
				pTriInfos[f].assignedGroup[i] = &pGroups[iNrActiveGroups]
				pTriInfos[f].assignedGroup[i].iVertexRepresentitive = vertIndex
				pTriInfos[f].assignedGroup[i].bOrientPreservering = (pTriInfos[f].iFlag & orientPreserving) != 0
				pTriInfos[f].assignedGroup[i].iNrFaces = 0
				pTriInfos[f].assignedGroup[i].pFaceIndices = piGroupTrianglesBuffer[iOffset:]
				iNrActiveGroups++

				addTriToGroup(pTriInfos[f].assignedGroup[i], f)
				bOrPre := (pTriInfos[f].iFlag & orientPreserving) != 0
				neighIndexL := pTriInfos[f].faceNeighbors[i]
				var neighIndexR int
				if i > 0 {
					neighIndexR = pTriInfos[f].faceNeighbors[i-1]
				} else {
					neighIndexR = pTriInfos[f].faceNeighbors[2]
				}
				if neighIndexL >= 0 {
					assignRecur(piTriListIn, pTriInfos, neighIndexL, pTriInfos[f].assignedGroup[i])
				}
				if neighIndexR >= 0 {
					assignRecur(piTriListIn, pTriInfos, neighIndexR, pTriInfos[f].assignedGroup[i])
				}
				_ = bOrPre // used in C for debug assertions only

				iOffset += pTriInfos[f].assignedGroup[i].iNrFaces
			}
		}
	}

	return iNrActiveGroups
}

// C lines 1133-1137
func addTriToGroup(pGroup *sGroup, iTriIndex int) {
	pGroup.pFaceIndices[pGroup.iNrFaces] = iTriIndex
	pGroup.iNrFaces++
}

// C lines 1139-1189
func assignRecur(piTriListIn []int, psTriInfos []sTriInfo, iMyTriIndex int, pGroup *sGroup) bool {
	pMyTriInfo := &psTriInfos[iMyTriIndex]

	iVertRep := pGroup.iVertexRepresentitive
	pVerts := piTriListIn[3*iMyTriIndex : 3*iMyTriIndex+3]
	i := -1
	if pVerts[0] == iVertRep {
		i = 0
	} else if pVerts[1] == iVertRep {
		i = 1
	} else if pVerts[2] == iVertRep {
		i = 2
	}
	if i < 0 {
		// vertex not found in triangle — should not happen
		return false
	}

	// early out
	if pMyTriInfo.assignedGroup[i] == pGroup {
		return true
	} else if pMyTriInfo.assignedGroup[i] != nil {
		return false
	}

	if (pMyTriInfo.iFlag & groupWithAny) != 0 {
		if pMyTriInfo.assignedGroup[0] == nil &&
			pMyTriInfo.assignedGroup[1] == nil &&
			pMyTriInfo.assignedGroup[2] == nil {
			pMyTriInfo.iFlag &= ^orientPreserving
			if pGroup.bOrientPreservering {
				pMyTriInfo.iFlag |= orientPreserving
			}
		}
	}

	bOrient := (pMyTriInfo.iFlag & orientPreserving) != 0
	if bOrient != pGroup.bOrientPreservering {
		return false
	}

	addTriToGroup(pGroup, iMyTriIndex)
	pMyTriInfo.assignedGroup[i] = pGroup

	neighIndexL := pMyTriInfo.faceNeighbors[i]
	var neighIndexR int
	if i > 0 {
		neighIndexR = pMyTriInfo.faceNeighbors[i-1]
	} else {
		neighIndexR = pMyTriInfo.faceNeighbors[2]
	}
	if neighIndexL >= 0 {
		assignRecur(piTriListIn, psTriInfos, neighIndexL, pGroup)
	}
	if neighIndexR >= 0 {
		assignRecur(piTriListIn, psTriInfos, neighIndexR, pGroup)
	}

	return true
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add build4RuleGroups, assignRecur, addTriToGroup"
```

---

### Task 7: Tangent space generation

**Files:**
- Modify: `mikktspace.go`

Translate C lines 1194-1493: `generateTSpaces`, `evalTspace`, `compareSubGroups`, `quickSort`.

- [ ] **Step 1: Replace the generateTSpaces stub and add supporting functions**

Replace the `generateTSpaces` stub:

```go
// C lines 1198-1365
func generateTSpaces(psTspace []sTSpace, pTriInfos []sTriInfo, pGroups []sGroup, iNrActiveGroups int, piTriListIn []int, fThresCos float32, ctx *context) bool {
	iMaxNrFaces := 0
	for g := 0; g < iNrActiveGroups; g++ {
		if iMaxNrFaces < pGroups[g].iNrFaces {
			iMaxNrFaces = pGroups[g].iNrFaces
		}
	}

	if iMaxNrFaces == 0 {
		return true
	}

	pSubGroupTspace := make([]sTSpace, iMaxNrFaces)
	pUniSubGroups := make([]sSubGroup, iMaxNrFaces)
	pTmpMembers := make([]int, iMaxNrFaces)

	for g := 0; g < iNrActiveGroups; g++ {
		pGroup := &pGroups[g]
		iUniqueSubGroups := 0

		for i := 0; i < pGroup.iNrFaces; i++ {
			f := pGroup.pFaceIndices[i]
			index := -1
			if pTriInfos[f].assignedGroup[0] == pGroup {
				index = 0
			} else if pTriInfos[f].assignedGroup[1] == pGroup {
				index = 1
			} else if pTriInfos[f].assignedGroup[2] == pGroup {
				index = 2
			}

			iVertIndex := piTriListIn[f*3+index]
			_ = iVertIndex // used for assertion in C

			// is normalized already
			n := getNormal(ctx, piTriListIn[f*3+index])

			// project
			vOs := vsub(pTriInfos[f].vOs, vscale(vdot(n, pTriInfos[f].vOs), n))
			vOt := vsub(pTriInfos[f].vOt, vscale(vdot(n, pTriInfos[f].vOt), n))
			if vNotZero(vOs) {
				vOs = vnormalize(vOs)
			}
			if vNotZero(vOt) {
				vOt = vnormalize(vOt)
			}

			iOF_1 := pTriInfos[f].iOrgFaceNumber

			iMembers := 0
			for j := 0; j < pGroup.iNrFaces; j++ {
				t := pGroup.pFaceIndices[j]
				iOF_2 := pTriInfos[t].iOrgFaceNumber

				vOs2 := vsub(pTriInfos[t].vOs, vscale(vdot(n, pTriInfos[t].vOs), n))
				vOt2 := vsub(pTriInfos[t].vOt, vscale(vdot(n, pTriInfos[t].vOt), n))
				if vNotZero(vOs2) {
					vOs2 = vnormalize(vOs2)
				}
				if vNotZero(vOt2) {
					vOt2 = vnormalize(vOt2)
				}

				bAny := ((pTriInfos[f].iFlag | pTriInfos[t].iFlag) & groupWithAny) != 0
				bSameOrgFace := iOF_1 == iOF_2

				fCosS := vdot(vOs, vOs2)
				fCosT := vdot(vOt, vOt2)

				if bAny || bSameOrgFace || (fCosS > fThresCos && fCosT > fThresCos) {
					pTmpMembers[iMembers] = t
					iMembers++
				}
			}

			// sort pTmpMembers
			if iMembers > 1 {
				uSeed := uint32(internalRndSortSeed)
				quickSort(pTmpMembers, 0, iMembers-1, uSeed)
			}

			// look for an existing match
			tmpGroup := sSubGroup{iNrFaces: iMembers, pTriMembers: pTmpMembers}
			bFound := false
			l := 0
			for l < iUniqueSubGroups && !bFound {
				bFound = compareSubGroups(&tmpGroup, &pUniSubGroups[l])
				if !bFound {
					l++
				}
			}

			// if no match was found we allocate a new subgroup
			if !bFound {
				pIndices := make([]int, iMembers)
				copy(pIndices, pTmpMembers[:iMembers])
				pUniSubGroups[iUniqueSubGroups].iNrFaces = iMembers
				pUniSubGroups[iUniqueSubGroups].pTriMembers = pIndices
				pSubGroupTspace[iUniqueSubGroups] =
					evalTspace(pTmpMembers[:iMembers], iMembers, piTriListIn, pTriInfos, ctx, pGroup.iVertexRepresentitive)
				iUniqueSubGroups++
			}

			// output tspace
			iOffs := pTriInfos[f].iTSpacesOffs
			iVert := int(pTriInfos[f].vertNum[index])
			pTSOut := &psTspace[iOffs+iVert]
			if pTSOut.iCounter == 1 {
				*pTSOut = avgTSpace(pTSOut, &pSubGroupTspace[l])
				pTSOut.iCounter = 2
				pTSOut.bOrient = pGroup.bOrientPreservering
			} else {
				*pTSOut = pSubGroupTspace[l]
				pTSOut.iCounter = 1
				pTSOut.bOrient = pGroup.bOrientPreservering
			}
		}
	}

	return true
}

// C lines 1367-1439
func evalTspace(faceIndices []int, iFaces int, piTriListIn []int, pTriInfos []sTriInfo, ctx *context, iVertexRepresentitive int) sTSpace {
	var res sTSpace
	fAngleSum := float32(0)

	for face := 0; face < iFaces; face++ {
		f := faceIndices[face]

		if (pTriInfos[f].iFlag & groupWithAny) == 0 {
			i := -1
			if piTriListIn[3*f+0] == iVertexRepresentitive {
				i = 0
			} else if piTriListIn[3*f+1] == iVertexRepresentitive {
				i = 1
			} else if piTriListIn[3*f+2] == iVertexRepresentitive {
				i = 2
			}

			index := piTriListIn[3*f+i]
			n := getNormal(ctx, index)
			vOs := vsub(pTriInfos[f].vOs, vscale(vdot(n, pTriInfos[f].vOs), n))
			vOt := vsub(pTriInfos[f].vOt, vscale(vdot(n, pTriInfos[f].vOt), n))
			if vNotZero(vOs) {
				vOs = vnormalize(vOs)
			}
			if vNotZero(vOt) {
				vOt = vnormalize(vOt)
			}

			var i2Idx, i0Idx int
			if i < 2 {
				i2Idx = piTriListIn[3*f+(i+1)]
			} else {
				i2Idx = piTriListIn[3*f+0]
			}
			i1Idx := piTriListIn[3*f+i]
			if i > 0 {
				i0Idx = piTriListIn[3*f+(i-1)]
			} else {
				i0Idx = piTriListIn[3*f+2]
			}

			p0 := getPosition(ctx, i0Idx)
			p1 := getPosition(ctx, i1Idx)
			p2 := getPosition(ctx, i2Idx)
			v1 := vsub(p0, p1)
			v2 := vsub(p2, p1)

			// project
			v1 = vsub(v1, vscale(vdot(n, v1), n))
			if vNotZero(v1) {
				v1 = vnormalize(v1)
			}
			v2 = vsub(v2, vscale(vdot(n, v2), n))
			if vNotZero(v2) {
				v2 = vnormalize(v2)
			}

			// weight contribution by the angle between the two edge vectors
			fCos := vdot(v1, v2)
			if fCos > 1 {
				fCos = 1
			} else if fCos < -1 {
				fCos = -1
			}
			fAngle := float32(math.Acos(float64(fCos)))
			fMagS := pTriInfos[f].fMagS
			fMagT := pTriInfos[f].fMagT

			res.vOs = vadd(res.vOs, vscale(fAngle, vOs))
			res.vOt = vadd(res.vOt, vscale(fAngle, vOt))
			res.fMagS += fAngle * fMagS
			res.fMagT += fAngle * fMagT
			fAngleSum += fAngle
		}
	}

	// normalize
	if vNotZero(res.vOs) {
		res.vOs = vnormalize(res.vOs)
	}
	if vNotZero(res.vOt) {
		res.vOt = vnormalize(res.vOt)
	}
	if fAngleSum > 0 {
		res.fMagS /= fAngleSum
		res.fMagT /= fAngleSum
	}

	return res
}

// C lines 1441-1452
func compareSubGroups(pg1, pg2 *sSubGroup) bool {
	if pg1.iNrFaces != pg2.iNrFaces {
		return false
	}
	for i := 0; i < pg1.iNrFaces; i++ {
		if pg1.pTriMembers[i] != pg2.pTriMembers[i] {
			return false
		}
	}
	return true
}

// C lines 1454-1493
func quickSort(pSortBuffer []int, iLeft, iRight int, uSeed uint32) {
	// Random
	t := uSeed & 31
	t = (uSeed << t) | (uSeed >> (32 - t))
	uSeed = uSeed + t + 3

	iL := iLeft
	iR := iRight
	n := (iR - iL) + 1
	index := int(uSeed % uint32(n))

	iMid := pSortBuffer[index+iL]

	for {
		for pSortBuffer[iL] < iMid {
			iL++
		}
		for pSortBuffer[iR] > iMid {
			iR--
		}

		if iL <= iR {
			pSortBuffer[iL], pSortBuffer[iR] = pSortBuffer[iR], pSortBuffer[iL]
			iL++
			iR--
		}

		if iL > iR {
			break
		}
	}

	if iLeft < iR {
		quickSort(pSortBuffer, iLeft, iR, uSeed)
	}
	if iL < iRight {
		quickSort(pSortBuffer, iL, iRight, uSeed)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add generateTSpaces, evalTspace, compareSubGroups, quickSort"
```

---

### Task 8: Degenerate handling

**Files:**
- Modify: `mikktspace.go`

Translate C lines 1737-1899: `degenPrologue` and `degenEpilogue`.

- [ ] **Step 1: Replace the degenPrologue and degenEpilogue stubs**

Replace both stubs:

```go
// C lines 1737-1818
func degenPrologue(pTriInfos []sTriInfo, piTriListOut []int, iNrTrianglesIn int, iTotTris int) {
	// locate quads with only one good triangle
	t := 0
	for t < (iTotTris - 1) {
		iFO_a := pTriInfos[t].iOrgFaceNumber
		iFO_b := pTriInfos[t+1].iOrgFaceNumber
		if iFO_a == iFO_b { // this is a quad
			bIsDeg_a := (pTriInfos[t].iFlag & markDegenerate) != 0
			bIsDeg_b := (pTriInfos[t+1].iFlag & markDegenerate) != 0
			if bIsDeg_a != bIsDeg_b {
				pTriInfos[t].iFlag |= quadOneDegenTri
				pTriInfos[t+1].iFlag |= quadOneDegenTri
			}
			t += 2
		} else {
			t++
		}
	}

	// reorder list so all degen triangles are moved to the back
	// without reordering the good triangles
	iNextGoodTriangleSearchIndex := 1
	t = 0
	bStillFindingGoodOnes := true
	for t < iNrTrianglesIn && bStillFindingGoodOnes {
		bIsGood := (pTriInfos[t].iFlag & markDegenerate) == 0
		if bIsGood {
			if iNextGoodTriangleSearchIndex < (t + 2) {
				iNextGoodTriangleSearchIndex = t + 2
			}
		} else {
			// search for the first good triangle
			bJustADegenerate := true
			for bJustADegenerate && iNextGoodTriangleSearchIndex < iTotTris {
				bIsGood2 := (pTriInfos[iNextGoodTriangleSearchIndex].iFlag & markDegenerate) == 0
				if bIsGood2 {
					bJustADegenerate = false
				} else {
					iNextGoodTriangleSearchIndex++
				}
			}

			t0 := t
			t1 := iNextGoodTriangleSearchIndex
			iNextGoodTriangleSearchIndex++

			// swap triangle t0 and t1
			if !bJustADegenerate {
				for i := 0; i < 3; i++ {
					index := piTriListOut[t0*3+i]
					piTriListOut[t0*3+i] = piTriListOut[t1*3+i]
					piTriListOut[t1*3+i] = index
				}
				triInfo := pTriInfos[t0]
				pTriInfos[t0] = pTriInfos[t1]
				pTriInfos[t1] = triInfo
			} else {
				bStillFindingGoodOnes = false
			}
		}

		if bStillFindingGoodOnes {
			t++
		}
	}
}

// C lines 1820-1899
func degenEpilogue(psTspace []sTSpace, pTriInfos []sTriInfo, piTriListIn []int, ctx *context, iNrTrianglesIn int, iTotTris int) {
	// deal with degenerate triangles
	for t := iNrTrianglesIn; t < iTotTris; t++ {
		bSkip := (pTriInfos[t].iFlag & quadOneDegenTri) != 0

		if !bSkip {
			for i := 0; i < 3; i++ {
				index1 := piTriListIn[t*3+i]
				bNotFound := true
				j := 0
				for bNotFound && j < (3*iNrTrianglesIn) {
					index2 := piTriListIn[j]
					if index1 == index2 {
						bNotFound = false
					} else {
						j++
					}
				}

				if !bNotFound {
					iTri := j / 3
					iVert := j % 3
					iSrcVert := int(pTriInfos[iTri].vertNum[iVert])
					iSrcOffs := pTriInfos[iTri].iTSpacesOffs
					iDstVert := int(pTriInfos[t].vertNum[i])
					iDstOffs := pTriInfos[t].iTSpacesOffs

					psTspace[iDstOffs+iDstVert] = psTspace[iSrcOffs+iSrcVert]
				}
			}
		}
	}

	// deal with degenerate quads with one good triangle
	for t := 0; t < iNrTrianglesIn; t++ {
		if (pTriInfos[t].iFlag & quadOneDegenTri) != 0 {
			pV := pTriInfos[t].vertNum
			iFlag := (1 << pV[0]) | (1 << pV[1]) | (1 << pV[2])
			iMissingIndex := 0
			if (iFlag & 2) == 0 {
				iMissingIndex = 1
			} else if (iFlag & 4) == 0 {
				iMissingIndex = 2
			} else if (iFlag & 8) == 0 {
				iMissingIndex = 3
			}

			iOrgF := pTriInfos[t].iOrgFaceNumber
			vDstP := getPosition(ctx, makeIndex(iOrgF, iMissingIndex))
			bNotFound := true
			i := 0
			for bNotFound && i < 3 {
				iVert := int(pV[i])
				vSrcP := getPosition(ctx, makeIndex(iOrgF, iVert))
				if veq(vSrcP, vDstP) {
					iOffs := pTriInfos[t].iTSpacesOffs
					psTspace[iOffs+iMissingIndex] = psTspace[iOffs+iVert]
					bNotFound = false
				} else {
					i++
				}
			}
		}
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add mikktspace.go
git commit -m "feat: add degenPrologue and degenEpilogue"
```

---

### Task 9: C reference dumper

**Files:**
- Create: `cmd/gendump/main.c`
- Create: `cmd/gendump/Makefile`

Write a small C program using cgltf (header-only) to load `NormalTangentTest.glb`, run MikkTSpace, and dump the golden output to JSON.

- [ ] **Step 1: Download cgltf header**

```bash
cd /Users/wizard/workspace/gamedev/MikkTSpace-go
mkdir -p cmd/gendump
curl -sL https://raw.githubusercontent.com/jkuhlmann/cgltf/master/cgltf.h -o cmd/gendump/cgltf.h
```

- [ ] **Step 2: Create main.c**

Create `cmd/gendump/main.c`:

```c
#define CGLTF_IMPLEMENTATION
#include "cgltf.h"
#include "../../mikktspace_ref/mikktspace.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Mesh data extracted from glTF
typedef struct {
    float *positions;  // 3 floats per vertex
    float *normals;    // 3 floats per vertex
    float *texcoords;  // 2 floats per vertex
    uint32_t *indices; // 3 per triangle
    int num_vertices;
    int num_triangles;
} Mesh;

// Output collection
typedef struct {
    int face;
    int vert;
    float tangent[3];
    float sign;
    float bitangent[3];
    float magS, magT;
    int orientationPreserving;
} TSpaceEntry;

static Mesh g_mesh;
static TSpaceEntry *g_entries;
static int g_num_entries;
static int g_entries_cap;

static void add_entry(TSpaceEntry e) {
    if (g_num_entries >= g_entries_cap) {
        g_entries_cap = g_entries_cap == 0 ? 1024 : g_entries_cap * 2;
        g_entries = realloc(g_entries, sizeof(TSpaceEntry) * g_entries_cap);
    }
    g_entries[g_num_entries++] = e;
}

// MikkTSpace callbacks
static int getNumFaces(const SMikkTSpaceContext *ctx) {
    return g_mesh.num_triangles;
}

static int getNumVerticesOfFace(const SMikkTSpaceContext *ctx, int iFace) {
    return 3;
}

static void getPosition(const SMikkTSpaceContext *ctx, float fvPosOut[], int iFace, int iVert) {
    uint32_t idx = g_mesh.indices[iFace * 3 + iVert];
    fvPosOut[0] = g_mesh.positions[idx * 3 + 0];
    fvPosOut[1] = g_mesh.positions[idx * 3 + 1];
    fvPosOut[2] = g_mesh.positions[idx * 3 + 2];
}

static void getNormal(const SMikkTSpaceContext *ctx, float fvNormOut[], int iFace, int iVert) {
    uint32_t idx = g_mesh.indices[iFace * 3 + iVert];
    fvNormOut[0] = g_mesh.normals[idx * 3 + 0];
    fvNormOut[1] = g_mesh.normals[idx * 3 + 1];
    fvNormOut[2] = g_mesh.normals[idx * 3 + 2];
}

static void getTexCoord(const SMikkTSpaceContext *ctx, float fvTexcOut[], int iFace, int iVert) {
    uint32_t idx = g_mesh.indices[iFace * 3 + iVert];
    fvTexcOut[0] = g_mesh.texcoords[idx * 2 + 0];
    fvTexcOut[1] = g_mesh.texcoords[idx * 2 + 1];
}

static void setTSpaceBasic(const SMikkTSpaceContext *ctx, const float fvTangent[], float fSign, int iFace, int iVert) {
    // find existing entry and update basic fields
    for (int i = g_num_entries - 1; i >= 0; i--) {
        if (g_entries[i].face == iFace && g_entries[i].vert == iVert) {
            g_entries[i].tangent[0] = fvTangent[0];
            g_entries[i].tangent[1] = fvTangent[1];
            g_entries[i].tangent[2] = fvTangent[2];
            g_entries[i].sign = fSign;
            return;
        }
    }
    // not found - create new
    TSpaceEntry e = {0};
    e.face = iFace;
    e.vert = iVert;
    e.tangent[0] = fvTangent[0];
    e.tangent[1] = fvTangent[1];
    e.tangent[2] = fvTangent[2];
    e.sign = fSign;
    add_entry(e);
}

static void setTSpace(const SMikkTSpaceContext *ctx, const float fvTangent[], const float fvBiTangent[],
                       float fMagS, float fMagT, tbool bIsOrientationPreserving, int iFace, int iVert) {
    TSpaceEntry e = {0};
    e.face = iFace;
    e.vert = iVert;
    e.tangent[0] = fvTangent[0];
    e.tangent[1] = fvTangent[1];
    e.tangent[2] = fvTangent[2];
    e.sign = bIsOrientationPreserving ? 1.0f : -1.0f;
    e.bitangent[0] = fvBiTangent[0];
    e.bitangent[1] = fvBiTangent[1];
    e.bitangent[2] = fvBiTangent[2];
    e.magS = fMagS;
    e.magT = fMagT;
    e.orientationPreserving = bIsOrientationPreserving;
    add_entry(e);
}

static float *read_float_accessor(cgltf_accessor *accessor) {
    float *data = malloc(sizeof(float) * accessor->count * cgltf_num_components(accessor->type));
    if (cgltf_accessor_unpack_floats(accessor, data, accessor->count * cgltf_num_components(accessor->type)) == 0) {
        free(data);
        return NULL;
    }
    return data;
}

static uint32_t *read_index_accessor(cgltf_accessor *accessor) {
    uint32_t *data = malloc(sizeof(uint32_t) * accessor->count);
    for (cgltf_size i = 0; i < accessor->count; i++) {
        data[i] = (uint32_t)cgltf_accessor_read_index(accessor, i);
    }
    return data;
}

int main(int argc, char **argv) {
    if (argc < 3) {
        fprintf(stderr, "Usage: %s <input.glb> <output.json>\n", argv[0]);
        return 1;
    }

    cgltf_options options = {0};
    cgltf_data *data = NULL;
    cgltf_result result = cgltf_parse_file(&options, argv[1], &data);
    if (result != cgltf_result_success) {
        fprintf(stderr, "Failed to parse glTF file\n");
        return 1;
    }
    result = cgltf_load_buffers(&options, data, argv[1]);
    if (result != cgltf_result_success) {
        fprintf(stderr, "Failed to load buffers\n");
        cgltf_free(data);
        return 1;
    }

    // Extract first mesh primitive
    cgltf_primitive *prim = &data->meshes[0].primitives[0];

    for (cgltf_size i = 0; i < prim->attributes_count; i++) {
        cgltf_attribute *attr = &prim->attributes[i];
        if (attr->type == cgltf_attribute_type_position) {
            g_mesh.positions = read_float_accessor(attr->data);
            g_mesh.num_vertices = (int)attr->data->count;
        } else if (attr->type == cgltf_attribute_type_normal) {
            g_mesh.normals = read_float_accessor(attr->data);
        } else if (attr->type == cgltf_attribute_type_texcoord && attr->index == 0) {
            g_mesh.texcoords = read_float_accessor(attr->data);
        }
    }
    g_mesh.indices = read_index_accessor(prim->indices);
    g_mesh.num_triangles = (int)prim->indices->count / 3;

    printf("Loaded mesh: %d vertices, %d triangles\n", g_mesh.num_vertices, g_mesh.num_triangles);

    // Run MikkTSpace
    SMikkTSpaceInterface iface = {0};
    iface.m_getNumFaces = getNumFaces;
    iface.m_getNumVerticesOfFace = getNumVerticesOfFace;
    iface.m_getPosition = getPosition;
    iface.m_getNormal = getNormal;
    iface.m_getTexCoord = getTexCoord;
    iface.m_setTSpace = setTSpace;
    iface.m_setTSpaceBasic = setTSpaceBasic;

    SMikkTSpaceContext context = {0};
    context.m_pInterface = &iface;

    tbool ok = genTangSpaceDefault(&context);
    printf("genTangSpaceDefault returned: %d\n", ok);
    printf("Collected %d entries\n", g_num_entries);

    // Write JSON
    FILE *fp = fopen(argv[2], "w");
    fprintf(fp, "[\n");
    for (int i = 0; i < g_num_entries; i++) {
        TSpaceEntry *e = &g_entries[i];
        fprintf(fp, "  {\"face\":%d,\"vert\":%d,"
                     "\"tangent\":[%.9g,%.9g,%.9g],\"sign\":%.9g,"
                     "\"bitangent\":[%.9g,%.9g,%.9g],"
                     "\"magS\":%.9g,\"magT\":%.9g,"
                     "\"orientationPreserving\":%s}%s\n",
                e->face, e->vert,
                e->tangent[0], e->tangent[1], e->tangent[2], e->sign,
                e->bitangent[0], e->bitangent[1], e->bitangent[2],
                e->magS, e->magT,
                e->orientationPreserving ? "true" : "false",
                (i < g_num_entries - 1) ? "," : "");
    }
    fprintf(fp, "]\n");
    fclose(fp);

    printf("Wrote %s\n", argv[2]);

    free(g_mesh.positions);
    free(g_mesh.normals);
    free(g_mesh.texcoords);
    free(g_mesh.indices);
    free(g_entries);
    cgltf_free(data);
    return 0;
}
```

Note: The C source expects `mikktspace.h` and `mikktspace.c` from the reference directory. We'll symlink or copy them.

- [ ] **Step 3: Create Makefile**

Create `cmd/gendump/Makefile`:

```makefile
MIKKT_DIR = ../../../MikkTSpace

gendump: main.c cgltf.h $(MIKKT_DIR)/mikktspace.c $(MIKKT_DIR)/mikktspace.h
	$(CC) -O2 -o $@ main.c $(MIKKT_DIR)/mikktspace.c -I$(MIKKT_DIR) -lm

clean:
	rm -f gendump
```

- [ ] **Step 4: Commit**

```bash
git add cmd/gendump/main.c cmd/gendump/Makefile
git commit -m "feat: add C reference dumper for golden output generation"
```

Note: `cgltf.h` is a downloaded dependency (~500KB), add to `.gitignore` or commit — user preference.

---

### Task 10: Generate golden test data

**Files:**
- Create: `testdata/` (copy test asset + generate golden output)

- [ ] **Step 1: Copy NormalTangentTest.glb to testdata**

```bash
cd /Users/wizard/workspace/gamedev/MikkTSpace-go
mkdir -p testdata
cp /Users/wizard/workspace/gata/glTF-Sample-Assets/Models/NormalTangentTest/glTF-Binary/NormalTangentTest.glb testdata/
```

- [ ] **Step 2: Build and run the C reference dumper**

```bash
cd /Users/wizard/workspace/gamedev/MikkTSpace-go/cmd/gendump
make
./gendump ../../testdata/NormalTangentTest.glb ../../testdata/golden.json
```

Expected: Output like "Loaded mesh: 3983 vertices, 7774 triangles" + "genTangSpaceDefault returned: 1" + "Wrote ../../testdata/golden.json"

- [ ] **Step 3: Verify golden.json looks reasonable**

```bash
head -5 /Users/wizard/workspace/gamedev/MikkTSpace-go/testdata/golden.json
wc -l /Users/wizard/workspace/gamedev/MikkTSpace-go/testdata/golden.json
```

Expected: JSON array of entries, thousands of lines (one per face*vert callback).

- [ ] **Step 4: Commit test data**

```bash
cd /Users/wizard/workspace/gamedev/MikkTSpace-go
git add testdata/NormalTangentTest.glb testdata/golden.json
git commit -m "feat: add test model and golden reference output"
```

---

### Task 11: Go test with minimal glTF reader

**Files:**
- Create: `mikktspace_test.go`

Write a test that loads `NormalTangentTest.glb` with a minimal inline glTF reader, runs `GenTangSpaceDefault`, and compares output against `golden.json`.

- [ ] **Step 1: Create mikktspace_test.go**

```go
package mikktspace

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
)

// Minimal glTF binary reader — just enough to extract positions, normals, UVs, indices.

type glbHeader struct {
	Magic   uint32
	Version uint32
	Length  uint32
}

type glbChunkHeader struct {
	ChunkLength uint32
	ChunkType   uint32
}

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
	Indices    int            `json:"indices"`
}

type testMesh struct {
	positions []float32 // 3 per vertex
	normals   []float32 // 3 per vertex
	uvs       []float32 // 2 per vertex
	indices   []uint32
}

func loadGLB(path string) (*testMesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read GLB header
	var hdr glbHeader
	if err := binary.Read(f, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Magic != 0x46546C67 { // "glTF"
		return nil, fmt.Errorf("not a GLB file")
	}

	// Read JSON chunk
	var jsonChunkHdr glbChunkHeader
	if err := binary.Read(f, binary.LittleEndian, &jsonChunkHdr); err != nil {
		return nil, err
	}
	jsonData := make([]byte, jsonChunkHdr.ChunkLength)
	if _, err := io.ReadFull(f, jsonData); err != nil {
		return nil, err
	}

	var gltf gltfJSON
	if err := json.Unmarshal(jsonData, &gltf); err != nil {
		return nil, err
	}

	// Read BIN chunk
	var binChunkHdr glbChunkHeader
	if err := binary.Read(f, binary.LittleEndian, &binChunkHdr); err != nil {
		return nil, err
	}
	binData := make([]byte, binChunkHdr.ChunkLength)
	if _, err := io.ReadFull(f, binData); err != nil {
		return nil, err
	}

	readFloats := func(accIdx int) []float32 {
		acc := gltf.Accessors[accIdx]
		bv := gltf.BufferViews[acc.BufferView]
		offset := bv.ByteOffset + acc.ByteOffset
		var numComponents int
		switch acc.Type {
		case "VEC2":
			numComponents = 2
		case "VEC3":
			numComponents = 3
		case "VEC4":
			numComponents = 4
		case "SCALAR":
			numComponents = 1
		}
		stride := bv.ByteStride
		if stride == 0 {
			stride = numComponents * 4
		}
		result := make([]float32, acc.Count*numComponents)
		for i := 0; i < acc.Count; i++ {
			base := offset + i*stride
			for c := 0; c < numComponents; c++ {
				bits := binary.LittleEndian.Uint32(binData[base+c*4:])
				result[i*numComponents+c] = math.Float32frombits(bits)
			}
		}
		return result
	}

	readIndices := func(accIdx int) []uint32 {
		acc := gltf.Accessors[accIdx]
		bv := gltf.BufferViews[acc.BufferView]
		offset := bv.ByteOffset + acc.ByteOffset
		result := make([]uint32, acc.Count)
		for i := 0; i < acc.Count; i++ {
			switch acc.ComponentType {
			case 5121: // UNSIGNED_BYTE
				result[i] = uint32(binData[offset+i])
			case 5123: // UNSIGNED_SHORT
				result[i] = uint32(binary.LittleEndian.Uint16(binData[offset+i*2:]))
			case 5125: // UNSIGNED_INT
				result[i] = binary.LittleEndian.Uint32(binData[offset+i*4:])
			}
		}
		return result
	}

	prim := gltf.Meshes[0].Primitives[0]
	mesh := &testMesh{
		positions: readFloats(prim.Attributes["POSITION"]),
		normals:   readFloats(prim.Attributes["NORMAL"]),
		uvs:       readFloats(prim.Attributes["TEXCOORD_0"]),
		indices:   readIndices(prim.Indices),
	}
	return mesh, nil
}

// Golden output entry
type goldenEntry struct {
	Face                   int        `json:"face"`
	Vert                   int        `json:"vert"`
	Tangent                [3]float64 `json:"tangent"`
	Sign                   float64    `json:"sign"`
	Bitangent              [3]float64 `json:"bitangent"`
	MagS                   float64    `json:"magS"`
	MagT                   float64    `json:"magT"`
	OrientationPreserving  bool       `json:"orientationPreserving"`
}

func loadGolden(path string) ([]goldenEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []goldenEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func TestGoldenComparison(t *testing.T) {
	mesh, err := loadGLB("testdata/NormalTangentTest.glb")
	if err != nil {
		t.Fatalf("failed to load GLB: %v", err)
	}

	golden, err := loadGolden("testdata/golden.json")
	if err != nil {
		t.Fatalf("failed to load golden: %v", err)
	}

	numTriangles := len(mesh.indices) / 3

	// Collect setTSpace results
	type tspaceResult struct {
		face      int
		vert      int
		tangent   [3]float32
		sign      float32
		bitangent [3]float32
		magS      float32
		magT      float32
		orient    bool
	}
	var results []tspaceResult

	getNumFaces := func() int { return numTriangles }
	getNumVerticesOfFace := func(face int) int { return 3 }
	getPosition := func(face, vert int) [3]float32 {
		idx := mesh.indices[face*3+vert]
		return [3]float32{
			mesh.positions[idx*3+0],
			mesh.positions[idx*3+1],
			mesh.positions[idx*3+2],
		}
	}
	getNormal := func(face, vert int) [3]float32 {
		idx := mesh.indices[face*3+vert]
		return [3]float32{
			mesh.normals[idx*3+0],
			mesh.normals[idx*3+1],
			mesh.normals[idx*3+2],
		}
	}
	getTexCoord := func(face, vert int) [2]float32 {
		idx := mesh.indices[face*3+vert]
		return [2]float32{
			mesh.uvs[idx*2+0],
			mesh.uvs[idx*2+1],
		}
	}
	setTSpaceBasic := func(face, vert int, tangent [3]float32, sign float32) {
		// update existing entry (setTSpace is called first)
		for i := len(results) - 1; i >= 0; i-- {
			if results[i].face == face && results[i].vert == vert {
				results[i].tangent = tangent
				results[i].sign = sign
				return
			}
		}
		results = append(results, tspaceResult{face: face, vert: vert, tangent: tangent, sign: sign})
	}
	setTSpace := func(face, vert int, tangent, bitangent [3]float32, magS, magT float32, orient bool) {
		results = append(results, tspaceResult{
			face: face, vert: vert,
			tangent: tangent, bitangent: bitangent,
			magS: magS, magT: magT, orient: orient,
			sign: func() float32 { if orient { return 1 }; return -1 }(),
		})
	}

	ok := GenTangSpaceDefault(getNumFaces, getNumVerticesOfFace, getPosition, getNormal, getTexCoord, setTSpaceBasic, setTSpace)
	if !ok {
		t.Fatal("GenTangSpaceDefault returned false")
	}

	if len(results) != len(golden) {
		t.Fatalf("entry count mismatch: got %d, want %d", len(results), len(golden))
	}

	const eps = 1e-5
	approxEq := func(a float32, b float64) bool {
		return math.Abs(float64(a)-b) < eps
	}

	mismatches := 0
	for i := range golden {
		g := golden[i]
		r := results[i]

		if r.face != g.Face || r.vert != g.Vert {
			t.Errorf("entry %d: face/vert mismatch: got (%d,%d), want (%d,%d)", i, r.face, r.vert, g.Face, g.Vert)
			mismatches++
			continue
		}

		if !approxEq(r.tangent[0], g.Tangent[0]) || !approxEq(r.tangent[1], g.Tangent[1]) || !approxEq(r.tangent[2], g.Tangent[2]) {
			t.Errorf("entry %d (face=%d,vert=%d): tangent mismatch: got [%v,%v,%v], want [%v,%v,%v]",
				i, r.face, r.vert, r.tangent[0], r.tangent[1], r.tangent[2], g.Tangent[0], g.Tangent[1], g.Tangent[2])
			mismatches++
		}
		if !approxEq(r.sign, g.Sign) {
			t.Errorf("entry %d (face=%d,vert=%d): sign mismatch: got %v, want %v", i, r.face, r.vert, r.sign, g.Sign)
			mismatches++
		}
		if !approxEq(r.bitangent[0], g.Bitangent[0]) || !approxEq(r.bitangent[1], g.Bitangent[1]) || !approxEq(r.bitangent[2], g.Bitangent[2]) {
			t.Errorf("entry %d (face=%d,vert=%d): bitangent mismatch", i, r.face, r.vert)
			mismatches++
		}
		if !approxEq(r.magS, g.MagS) || !approxEq(r.magT, g.MagT) {
			t.Errorf("entry %d (face=%d,vert=%d): magnitude mismatch", i, r.face, r.vert)
			mismatches++
		}

		if mismatches > 20 {
			t.Fatalf("too many mismatches, stopping early")
		}
	}

	if mismatches == 0 {
		t.Logf("All %d entries match golden output within epsilon %v", len(results), eps)
	}
}
```

- [ ] **Step 2: Verify the test compiles (will fail until golden data exists)**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go vet ./...`
Expected: Success (compiles, vet passes)

- [ ] **Step 3: Run the test**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go test -v -run TestGoldenComparison -count=1`
Expected: PASS (if golden data exists and port is correct)

If mismatches occur, debug by comparing the first failing entry. Common issues:
- `notZero` implementation: verify the FLT_MIN threshold matches C exactly
- `float32` vs `float64` intermediate precision: ensure all math uses `float32` like C
- Edge sort order: verify `quickSortEdges` produces same order as C

- [ ] **Step 4: Commit**

```bash
git add mikktspace_test.go
git commit -m "feat: add golden comparison test with minimal glTF reader"
```

---

### Task 12: Final verification and cleanup

**Files:**
- Verify all files compile and tests pass

- [ ] **Step 1: Run full build**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go build ./...`
Expected: Success

- [ ] **Step 2: Run full test suite**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go test -v -count=1 ./...`
Expected: PASS

- [ ] **Step 3: Run go vet**

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && go vet ./...`
Expected: No issues

- [ ] **Step 4: Verify no unused stubs remain**

Search for any remaining `panic("not yet implemented")` calls:

Run: `cd /Users/wizard/workspace/gamedev/MikkTSpace-go && grep -r "not yet implemented" mikktspace.go`
Expected: No output (all stubs replaced)

- [ ] **Step 5: Commit final state**

```bash
git add -A
git commit -m "chore: final verification — all tests passing"
```
