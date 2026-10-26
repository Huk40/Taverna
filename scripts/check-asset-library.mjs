// Run against a local development backend: node --env-file=.env scripts/check-asset-library.mjs
// Only the test's own generated asset and project are deleted.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { adminSession } from "./admin-session.mjs";

const origin = process.env.TEST_API_URL ?? "http://localhost:8080";
const auth = await adminSession(origin);
let assetID;
let projectID;
async function call(path, options, expected = 200) {
  const response = await fetch(origin + path, { ...options, headers: { "X-Taverna-Request": "1", ...options?.headers } });
  assert.equal(response.status, expected, `${options?.method ?? "GET"} ${path}: ${await response.clone().text()}`);
  return response.status === 204 ? null : response.json();
}
const json = (value) => ({ "Content-Type": "application/json", ...value });
try {
  await call("/api/auth/me", {}, 401);
  assert.equal((await call("/api/auth/me", { headers: auth })).user.role, "admin");
  const file = await readFile(new URL("../tests/fixtures/library-sword.obj", import.meta.url));
  const data = new FormData();
  data.append("file", new Blob([file]), "library-sword.obj");
  data.append("label", `Asset library check ${Date.now()}`);
  data.append("slot", "gear");
  data.append("attachment_point", "right_hand");
  await call("/api/model-assets", { method: "POST", body: data }, 401);
  const asset = await call("/api/model-assets", { method: "POST", body: data, headers: auth }, 201);
  assetID = asset.id;
  assert.equal(asset.metadata.status, "parsed");
  assert.ok(asset.metadata.triangle_count > 0);
  const library = await call("/api/model-assets?slot=gear");
  assert.ok(library.some((item) => item.id === assetID));
  assert.ok(!(await call("/api/model-assets?slot=head")).some((item) => item.id === assetID));
  const downloaded = await fetch(origin + asset.url);
  assert.equal(downloaded.status, 200);
  assert.equal(await downloaded.text(), file.toString());
  await call(`/api/model-assets/${assetID}`, { method: "PUT", headers: json(), body: JSON.stringify({ label: "Unauthorized" }) }, 401);
  await call(`/api/model-assets/${assetID}`, { method: "DELETE" }, 401);
  const updated = await call(`/api/model-assets/${assetID}`, { method: "PUT", headers: json(auth), body: JSON.stringify({ label: "Updated test sword" }) });
  assert.equal(updated.label, "Updated test sword");
  const catalog = await call("/api/catalog");
  const config = { ...catalog.defaults, assembly: { placements: [{
    id: "test-placement", slot: "gear", attachment_point: "right_hand", asset_id: assetID,
    asset_point_id: asset.assembly_points[0].id,
    transform: { position: [0, 0, 0], rotation: [0, 0, 0], scale: 1 },
  }] } };
  const manifest = await call("/api/assembly/preview", { method: "POST", headers: json(), body: JSON.stringify({ config }) });
  assert.equal(manifest.placements[0].asset.id, assetID);
  assert.ok(manifest.placements[0].world_bounds);
  const project = await call("/api/projects", { method: "POST", headers: json(auth), body: JSON.stringify({ name: "Asset library integration check", config }) }, 201);
  projectID = project.id;
  assert.equal((await call(`/api/projects/${projectID}`, { headers: auth })).config.assembly.placements[0].asset_id, assetID);
  await call(`/api/model-assets/${assetID}`, { method: "DELETE", headers: auth }, 409);
  await call(`/api/model-assets/${assetID}`, { method: "PUT", headers: json(auth), body: JSON.stringify({ assembly_points: [{ ...asset.assembly_points[0], id: "renamed-anchor" }] }) }, 409);
  console.log("PASS: admin access, upload, public library/file, category filters, update, assembly, save/reopen and reference protection");
} finally {
  if (projectID) await call(`/api/projects/${projectID}`, { method: "DELETE", headers: auth }, 204);
  if (assetID) await call(`/api/model-assets/${assetID}`, { method: "DELETE", headers: auth }, 204);
  await call("/api/auth/logout", { method: "POST", headers: auth }, 204);
}
