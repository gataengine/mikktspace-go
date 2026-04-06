// Package mikktspace is a faithful Go port of the MikkTSpace C library.
//
// Copyright (C) 2011 by Morten S. Mikkelsen
//
// This software is provided 'as-is', without any express or implied
// warranty. In no event will the authors be held liable for any damages
// arising from the use of this software.
//
// Permission is granted to anyone to use this software for any purpose,
// including commercial applications, and to alter it and redistribute it
// freely, subject to the following restrictions:
//
//  1. The origin of this software must not be misrepresented; you must not
//     claim that you wrote the original software. If you use this software
//     in a product, an acknowledgment in the product documentation would be
//     appreciated but is not required.
//  2. Altered source versions must be plainly marked as such, and must not be
//     misrepresented as being the original software.
//  3. This notice may not be removed or altered from any source distribution.
package mikktspace

import "math"

// flag constants (C lines 134-137)
const (
	markDegenerate   = 1
	quadOneDegenTri  = 2
	groupWithAny     = 4
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
	vOs      sVec3
	fMagS    float32
	vOt      sVec3
	fMagT    float32
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// generateInitialVerticesIndexList (C lines 772-881)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Vertex welding (C lines 429-770)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// initTriInfo, calcTexArea, and neighbor building (C lines 916-1061, 1501-1731)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

					var t0, t1v int
					if bChooseOrientFirstTri {
						t0 = t
						t1v = t + 1
					} else {
						t0 = t + 1
						t1v = t
					}
					pTriInfos[t1v].iFlag &= ^orientPreserving              // clear first
					pTriInfos[t1v].iFlag |= (pTriInfos[t0].iFlag & orientPreserving) // copy bit
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Group formation (C lines 1066-1189)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
		// vertex not found in triangle -- should not happen
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Tangent space generation (C lines 1194-1493)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Degenerate handling (C lines 1737-1899)
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
					idx := piTriListOut[t0*3+i]
					piTriListOut[t0*3+i] = piTriListOut[t1*3+i]
					piTriListOut[t1*3+i] = idx
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
				for bNotFound && j < (3 * iNrTrianglesIn) {
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
