// Read-only integration check for both bodies and all interchangeable head combinations.
import assert from "node:assert/strict";
const base = process.env.TEST_API_URL ?? "http://localhost:8080";
const assets = await (await fetch(base + "/api/model-assets")).json();
const catalog = await (await fetch(base + "/api/catalog")).json();
let checked = 0;
for (const bodyName of ["male-body.glb", "female-body.glb"]) {
  for (const headName of ["male-head.glb", "female-head.glb"]) {
    const body = assets.find((asset) => asset.file_name === bodyName);
    const head = assets.find((asset) => asset.file_name === headName);
    assert.ok(body && head, "Starter assets are missing");
    for (const mm of [32, 40]) {
      // Deliberately put the head before its body.
      const placements = [head, body].map((asset) => ({
        id: asset.id, slot: asset.slot, attachment_point: asset.attachment_point,
        asset_id: asset.id, asset_point_id: asset.assembly_points[0].id,
        transform: { position: [0, 0, 0], rotation: [0, 0, 0], scale: 1 },
      }));
      const response = await fetch(base + "/api/assembly/preview", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ config: { ...catalog.defaults, scale: { mm }, assembly: { placements } } }),
      });
      assert.equal(response.status, 200, await response.clone().text());
      const manifest = await response.json();
      const placedBody = manifest.placements.find((item) => item.slot === "body");
      const placedHead = manifest.placements.find((item) => item.slot === "head");
      const gap = placedHead.world_bounds.min[1] - placedBody.world_bounds.max[1];
      assert.ok(Math.abs(gap) < 0.0002, `Neck gap in ${bodyName}/${headName}: ${gap}`);
      assert.ok(Math.abs(placedBody.world_bounds.min[1] - 0.16 * manifest.character_scale) < 0.0002, "Feet miss the base");
      checked++;
    }
  }
}
console.log(`PASS: ${checked} body/head assemblies, interchangeable heads, matching necks and feet on the base`);
