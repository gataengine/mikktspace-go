/*
 * gendump - Generate MikkTSpace reference output from a glTF binary file.
 *
 * Usage: gendump input.glb output.json
 *
 * Loads the first mesh primitive from a .glb file, runs the original C
 * MikkTSpace library on it, and dumps all tangent-space results to JSON.
 * The JSON serves as "golden output" for the Go port's tests.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define CGLTF_IMPLEMENTATION
#include "cgltf.h"
#include "mikktspace.h"

/* ------------------------------------------------------------------ */
/* Mesh data extracted from glTF                                       */
/* ------------------------------------------------------------------ */

typedef struct {
    float *positions;   /* 3 floats per vertex */
    float *normals;     /* 3 floats per vertex */
    float *texcoords;   /* 2 floats per vertex */
    unsigned int *indices;
    int num_indices;
    int num_faces;
} Mesh;

/* ------------------------------------------------------------------ */
/* Per-(face,vert) tangent-space entry collected from callbacks         */
/* ------------------------------------------------------------------ */

typedef struct {
    int face;
    int vert;
    float tangent[3];
    float sign;
    float bitangent[3];
    float magS;
    float magT;
    int orientationPreserving;
} TSEntry;

typedef struct {
    Mesh *mesh;
    TSEntry *entries;
    int num_entries;
    int cap_entries;
} UserData;

/* ------------------------------------------------------------------ */
/* Helper: vertex index for (face, vert)                               */
/* ------------------------------------------------------------------ */

static int vertIndex(const Mesh *m, int iFace, int iVert)
{
    return m->indices[iFace * 3 + iVert];
}

/* ------------------------------------------------------------------ */
/* MikkTSpace callbacks                                                */
/* ------------------------------------------------------------------ */

static int cb_getNumFaces(const SMikkTSpaceContext *ctx)
{
    UserData *ud = (UserData *)ctx->m_pUserData;
    return ud->mesh->num_faces;
}

static int cb_getNumVerticesOfFace(const SMikkTSpaceContext *ctx, const int iFace)
{
    (void)ctx; (void)iFace;
    return 3;
}

static void cb_getPosition(const SMikkTSpaceContext *ctx, float fvPosOut[],
                            const int iFace, const int iVert)
{
    UserData *ud = (UserData *)ctx->m_pUserData;
    int vi = vertIndex(ud->mesh, iFace, iVert);
    fvPosOut[0] = ud->mesh->positions[vi * 3 + 0];
    fvPosOut[1] = ud->mesh->positions[vi * 3 + 1];
    fvPosOut[2] = ud->mesh->positions[vi * 3 + 2];
}

static void cb_getNormal(const SMikkTSpaceContext *ctx, float fvNormOut[],
                          const int iFace, const int iVert)
{
    UserData *ud = (UserData *)ctx->m_pUserData;
    int vi = vertIndex(ud->mesh, iFace, iVert);
    fvNormOut[0] = ud->mesh->normals[vi * 3 + 0];
    fvNormOut[1] = ud->mesh->normals[vi * 3 + 1];
    fvNormOut[2] = ud->mesh->normals[vi * 3 + 2];
}

static void cb_getTexCoord(const SMikkTSpaceContext *ctx, float fvTexcOut[],
                            const int iFace, const int iVert)
{
    UserData *ud = (UserData *)ctx->m_pUserData;
    int vi = vertIndex(ud->mesh, iFace, iVert);
    fvTexcOut[0] = ud->mesh->texcoords[vi * 2 + 0];
    fvTexcOut[1] = ud->mesh->texcoords[vi * 2 + 1];
}

/* setTSpace is called first: creates the entry with bitangent/magS/magT/orient */
static void cb_setTSpace(const SMikkTSpaceContext *ctx,
                          const float fvTangent[], const float fvBiTangent[],
                          const float fMagS, const float fMagT,
                          const tbool bIsOrientationPreserving,
                          const int iFace, const int iVert)
{
    UserData *ud = (UserData *)ctx->m_pUserData;

    if (ud->num_entries >= ud->cap_entries) {
        ud->cap_entries = ud->cap_entries ? ud->cap_entries * 2 : 1024;
        ud->entries = realloc(ud->entries, sizeof(TSEntry) * ud->cap_entries);
    }

    TSEntry *e = &ud->entries[ud->num_entries];
    e->face = iFace;
    e->vert = iVert;
    /* tangent will be filled by setTSpaceBasic */
    e->tangent[0] = fvTangent[0];
    e->tangent[1] = fvTangent[1];
    e->tangent[2] = fvTangent[2];
    e->sign = 0.0f; /* filled later */
    e->bitangent[0] = fvBiTangent[0];
    e->bitangent[1] = fvBiTangent[1];
    e->bitangent[2] = fvBiTangent[2];
    e->magS = fMagS;
    e->magT = fMagT;
    e->orientationPreserving = bIsOrientationPreserving ? 1 : 0;

    ud->num_entries++;
}

/* setTSpaceBasic is called second: updates the last entry with tangent and sign */
static void cb_setTSpaceBasic(const SMikkTSpaceContext *ctx,
                               const float fvTangent[], const float fSign,
                               const int iFace, const int iVert)
{
    UserData *ud = (UserData *)ctx->m_pUserData;

    /* Find the matching entry (should be the last one added by setTSpace) */
    int idx = ud->num_entries - 1;
    if (idx < 0 || ud->entries[idx].face != iFace || ud->entries[idx].vert != iVert) {
        fprintf(stderr, "setTSpaceBasic: no matching entry for face=%d vert=%d\n", iFace, iVert);
        return;
    }

    ud->entries[idx].tangent[0] = fvTangent[0];
    ud->entries[idx].tangent[1] = fvTangent[1];
    ud->entries[idx].tangent[2] = fvTangent[2];
    ud->entries[idx].sign = fSign;
}

/* ------------------------------------------------------------------ */
/* cgltf helpers                                                       */
/* ------------------------------------------------------------------ */

static float *read_accessor_floats(cgltf_accessor *acc, int components)
{
    cgltf_size count = acc->count;
    float *buf = malloc(sizeof(float) * count * components);
    for (cgltf_size i = 0; i < count; i++) {
        cgltf_accessor_read_float(acc, i, buf + i * components, components);
    }
    return buf;
}

static unsigned int *read_accessor_indices(cgltf_accessor *acc, int *out_count)
{
    cgltf_size count = acc->count;
    unsigned int *buf = malloc(sizeof(unsigned int) * count);
    for (cgltf_size i = 0; i < count; i++) {
        buf[i] = (unsigned int)cgltf_accessor_read_index(acc, i);
    }
    *out_count = (int)count;
    return buf;
}

/* ------------------------------------------------------------------ */
/* main                                                                */
/* ------------------------------------------------------------------ */

int main(int argc, char **argv)
{
    if (argc != 3) {
        fprintf(stderr, "Usage: %s input.glb output.json\n", argv[0]);
        return 1;
    }

    const char *glb_path = argv[1];
    const char *json_path = argv[2];

    /* Parse glTF */
    cgltf_options options = {0};
    cgltf_data *data = NULL;
    cgltf_result result = cgltf_parse_file(&options, glb_path, &data);
    if (result != cgltf_result_success) {
        fprintf(stderr, "cgltf_parse_file failed: %d\n", result);
        return 1;
    }

    result = cgltf_load_buffers(&options, data, glb_path);
    if (result != cgltf_result_success) {
        fprintf(stderr, "cgltf_load_buffers failed: %d\n", result);
        cgltf_free(data);
        return 1;
    }

    if (data->meshes_count == 0 || data->meshes[0].primitives_count == 0) {
        fprintf(stderr, "No mesh primitives found\n");
        cgltf_free(data);
        return 1;
    }

    cgltf_primitive *prim = &data->meshes[0].primitives[0];

    /* Extract attributes */
    Mesh mesh = {0};
    cgltf_accessor *pos_acc = NULL, *norm_acc = NULL, *tc_acc = NULL;

    for (cgltf_size i = 0; i < prim->attributes_count; i++) {
        cgltf_attribute *attr = &prim->attributes[i];
        if (attr->type == cgltf_attribute_type_position) pos_acc = attr->data;
        else if (attr->type == cgltf_attribute_type_normal) norm_acc = attr->data;
        else if (attr->type == cgltf_attribute_type_texcoord && attr->index == 0) tc_acc = attr->data;
    }

    if (!pos_acc || !norm_acc || !tc_acc) {
        fprintf(stderr, "Missing required attributes (position/normal/texcoord)\n");
        cgltf_free(data);
        return 1;
    }

    if (!prim->indices) {
        fprintf(stderr, "Mesh primitive has no indices\n");
        cgltf_free(data);
        return 1;
    }

    mesh.positions = read_accessor_floats(pos_acc, 3);
    mesh.normals = read_accessor_floats(norm_acc, 3);
    mesh.texcoords = read_accessor_floats(tc_acc, 2);
    mesh.indices = read_accessor_indices(prim->indices, &mesh.num_indices);
    mesh.num_faces = mesh.num_indices / 3;

    fprintf(stderr, "Loaded mesh: %d faces, %d indices\n", mesh.num_faces, mesh.num_indices);

    /* Set up MikkTSpace */
    UserData ud = {0};
    ud.mesh = &mesh;

    SMikkTSpaceInterface iface = {0};
    iface.m_getNumFaces = cb_getNumFaces;
    iface.m_getNumVerticesOfFace = cb_getNumVerticesOfFace;
    iface.m_getPosition = cb_getPosition;
    iface.m_getNormal = cb_getNormal;
    iface.m_getTexCoord = cb_getTexCoord;
    iface.m_setTSpace = cb_setTSpace;
    iface.m_setTSpaceBasic = cb_setTSpaceBasic;

    SMikkTSpaceContext context = {0};
    context.m_pInterface = &iface;
    context.m_pUserData = &ud;

    tbool ok = genTangSpaceDefault(&context);
    if (!ok) {
        fprintf(stderr, "genTangSpaceDefault failed\n");
        return 1;
    }

    fprintf(stderr, "Generated %d tangent-space entries\n", ud.num_entries);

    /* Dump to JSON */
    FILE *fp = fopen(json_path, "w");
    if (!fp) {
        fprintf(stderr, "Cannot open %s for writing\n", json_path);
        return 1;
    }

    fprintf(fp, "[\n");
    for (int i = 0; i < ud.num_entries; i++) {
        TSEntry *e = &ud.entries[i];
        fprintf(fp, "  {\"face\":%d,\"vert\":%d,"
                     "\"tangent\":[%.9g,%.9g,%.9g],"
                     "\"sign\":%.9g,"
                     "\"bitangent\":[%.9g,%.9g,%.9g],"
                     "\"magS\":%.9g,\"magT\":%.9g,"
                     "\"orientationPreserving\":%s}%s\n",
                e->face, e->vert,
                e->tangent[0], e->tangent[1], e->tangent[2],
                e->sign,
                e->bitangent[0], e->bitangent[1], e->bitangent[2],
                e->magS, e->magT,
                e->orientationPreserving ? "true" : "false",
                (i + 1 < ud.num_entries) ? "," : "");
    }
    fprintf(fp, "]\n");
    fclose(fp);

    fprintf(stderr, "Wrote %s\n", json_path);

    /* Cleanup */
    free(mesh.positions);
    free(mesh.normals);
    free(mesh.texcoords);
    free(mesh.indices);
    free(ud.entries);
    cgltf_free(data);

    return 0;
}
