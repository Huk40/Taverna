// Read-only API check: no project saves or asset changes.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
const api = process.env.TEST_API_URL ?? "http://localhost:8080";
const get = async (path) => { const response = await fetch(api + path); assert.equal(response.status, 200); return response.json(); };
const [catalog, assets] = await Promise.all([get("/api/catalog"), get("/api/model-assets")]);
assert.equal((await fetch(api + "/api/projects")).status, 401, "Guest projects must be private");
const output = new URL("../artifacts/stl/", import.meta.url);
await mkdir(output, { recursive: true });
const hashes = new Set();
let count = 0;
for (const bodyName of ["male-body.glb", "female-body.glb"]) {
  for (const headName of ["male-head.glb", "female-head.glb"]) {
    const chosen = [bodyName, headName].map((name) => assets.find((asset) => asset.file_name === name));
    assert.ok(chosen.every(Boolean), "Missing starter models");
    for (const height of [32, 40]) {
      const config = { ...catalog.defaults, scale: { mm: height }, assembly: { placements: chosen.map((asset) => ({
        id: asset.id, asset_id: asset.id, slot: asset.slot, attachment_point: asset.attachment_point,
        asset_point_id: asset.assembly_points[0].id, transform: { position: [0, 0, 0], rotation: [0, 0, 0], scale: 1 },
      })) } };
      const response = await fetch(api + "/api/exports/stl", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ config }) });
      assert.equal(response.status, 200, response.status === 200 ? "" : await response.text());
      assert.match(response.headers.get("Content-Type"), /model\/stl/);
      assert.match(response.headers.get("Content-Disposition"), new RegExp(`taverna-${height}mm\\.stl`));
      const data = Buffer.from(await response.arrayBuffer());
      const triangles = data.readUInt32LE(80);
      assert.equal(data.length, 84 + triangles * 50, "Invalid binary STL length");
      // Both GLBs plus the 64-segment closed base (4 triangles per segment).
      assert.ok(triangles >= 5400 && triangles <= 6100, `Missing/duplicated geometry: ${triangles} triangles`);
      let minZ = Infinity, maxZ = -Infinity, belowNeck = 0, aboveNeck = 0;
      for (let i = 0; i < triangles; i++) {
        const offset = 84 + i * 50;
        for (let component = 0; component < 12; component++) assert.ok(Number.isFinite(data.readFloatLE(offset + component * 4)), "Non-finite geometry");
        for (let vertex = 0; vertex < 3; vertex++) {
          const z = data.readFloatLE(offset + 12 + vertex * 12 + 8);
          minZ = Math.min(minZ, z); maxZ = Math.max(maxZ, z);
          if (z > height * 0.8) aboveNeck++; else if (z > height * 0.1) belowNeck++;
        }
      }
      assert.equal(minZ, 0, "Base is not on the print bed");
      assert.ok(Math.abs(maxZ - height) < 0.001, `Expected ${height} mm, got ${maxZ}`);
      assert.ok(aboveNeck > 100 && belowNeck > 100, "Missing head or body geometry");
      hashes.add(createHash("sha256").update(data).digest("hex"));
      if (height === 32 && bodyName.split("-")[0] === headName.split("-")[0]) {
        await writeFile(new URL(bodyName.replace("-body.glb", "-32mm.stl"), output), data);
      }
      count++;
    }
  }
}
assert.equal(hashes.size, count, "Selections or height changes are ignored");
const empty = await fetch(api + "/api/exports/stl", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ config: catalog.defaults }) });
assert.equal(empty.status, 400, "Empty assembly must not download a file");
assert.equal((await fetch(api + "/api/projects")).status, 401);
assert.equal((await get("/api/model-assets")).length, assets.length);
console.log(`PASS: ${count} guest STL downloads; all body/head combinations, 32/40 mm, Z-up, base included. Samples: artifacts/stl/`);
