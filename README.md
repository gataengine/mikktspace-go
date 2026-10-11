# MikkTSpace-go

A pure Go port of the [MikkTSpace](http://www.mikktspace.com/) library for computing tangent space on meshes. MikkTSpace is the industry standard used by Blender, Unity, Unreal, Filament, and others to ensure consistent tangent spaces across tools.

This is a faithful 1:1 translation of the [C implementation](https://github.com/mmikk/MikkTSpace) by Morten S. Mikkelsen. Given identical input, it produces bit-comparable output to the C original.

## Installation

```bash
go get github.com/gataengine/mikktspace-go
```

## Usage

```go
import "github.com/gataengine/mikktspace-go"

ok := mikktspace.GenTangSpaceDefault(
    func() int { return numTriangles },
    func(face int) int { return 3 },
    func(face, vert int) [3]float32 { return positions[indices[face*3+vert]] },
    func(face, vert int) [3]float32 { return normals[indices[face*3+vert]] },
    func(face, vert int) [2]float32 { return uvs[indices[face*3+vert]] },
    func(face, vert int, tangent [3]float32, sign float32) {
        // store tangent and sign
        // bitangent = sign * cross(normal, tangent)
    },
    nil, // setTSpace (optional, for full tangent/bitangent/magnitudes)
)
```

### API

**`GenTangSpaceDefault`** — Computes tangent space with smoothing disabled (angular threshold = 180 degrees). This is the recommended entry point.

**`GenTangSpace`** — Same as above but with a configurable angular threshold (0-180 degrees). Lower values produce harder edges.

Both functions take callback arguments:

| Callback | Required | Description |
|----------|----------|-------------|
| `getNumFaces` | yes | Total number of faces |
| `getNumVerticesOfFace` | yes | Return 3 (triangle) or 4 (quad) |
| `getPosition` | yes | Vertex position for (face, vert) |
| `getNormal` | yes | Vertex normal for (face, vert) |
| `getTexCoord` | yes | Texture coordinate for (face, vert) |
| `setTSpaceBasic` | at least one | Receives tangent (unit vector) + sign for bitangent reconstruction |
| `setTSpace` | at least one | Receives tangent, bitangent, magnitudes, and orientation flag |

Either `setTSpaceBasic` or `setTSpace` can be `nil`, but at least one must be provided.

Results are returned **unindexed** (one tangent per face-vertex pair). Do not average them using an existing index list.

### Quads

Quads are supported. Return 4 from `getNumVerticesOfFace` and provide vertex data for indices 0-3. Quads are internally split along the shortest diagonal for order-independent results.

## Properties

- **Order-independent** — faces and vertices can be provided in any order
- **Deterministic** — same input always produces the same output
- **Thread-safe** — no shared mutable state
- **Zero dependencies** — pure Go, stdlib only (`math` package)
- **Handles degenerate geometry** — degenerate triangles inherit tangent space from neighbors

## License

zlib/libpng license (same as the original C library). See the license header in `mikktspace.go`.
