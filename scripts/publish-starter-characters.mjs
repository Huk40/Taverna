// node --env-file=.env scripts/publish-starter-characters.mjs [--replace-library]
// The replacement option backs up and removes the pre-existing model library.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { adminSession } from "./admin-session.mjs";

const base = process.env.TEST_API_URL ?? "http://localhost:8080";
const headers = await adminSession(base);
const source = new URL("../assets/starter-characters/", import.meta.url);
const entries = JSON.parse(await readFile(new URL("manifest.json", source), "utf8"));
async function request(path, options = {}, status = 200) {
  const response = await fetch(base + path, options);
  assert.equal(response.status, status, `${options.method ?? "GET"} ${path}: ${await response.clone().text()}`);
  return status === 204 ? null : response.json();
}
const previous = await request("/api/model-assets");
const published = [];
for (const entry of entries) {
  const bytes = await readFile(new URL(entry.file, source));
  const hash = (data) => createHash("sha256").update(data).digest("hex");
  const existing = previous.find((asset) => asset.file_name === entry.file && asset.label === entry.label);
  let asset;
  if (existing) {
    const response = await fetch(base + existing.url);
    if (response.ok && hash(Buffer.from(await response.arrayBuffer())) === hash(bytes)) asset = existing;
  }
  if (!asset) {
    const data = new FormData();
    data.append("file", new Blob([bytes], { type: "model/gltf-binary" }), entry.file);
    data.append("label", entry.label);
    data.append("slot", entry.slot);
    data.append("attachment_point", entry.attachment_point);
    data.append("assembly_points", JSON.stringify(entry.assembly_points));
    asset = await request("/api/model-assets", { method: "POST", headers, body: data }, 201);
  }
  assert.equal(asset.metadata.status, "parsed");
  assert.equal(asset.metadata.triangle_count, entry.triangles);
  const downloaded = await fetch(base + asset.url);
  assert.equal(downloaded.status, 200);
  assert.equal(hash(Buffer.from(await downloaded.arrayBuffer())), hash(bytes));
  published.push(asset);
  console.log(`Ready: ${asset.label} (${asset.metadata.triangle_count} triangles)`);
}
await writeFile(process.env.PUBLISHED_MANIFEST_PATH ?? new URL("published.local.json", source), JSON.stringify(published, null, 2));

if (process.argv.includes("--replace-library")) {
  const remove = previous.filter((asset) => !published.some((item) => item.id === asset.id));
  // Complete backups before deleting any of the previous assets.
  const backup = new URL(`../backups/model-assets-${Date.now()}/`, import.meta.url);
  await mkdir(backup, { recursive: true });
  await writeFile(new URL("metadata.json", backup), JSON.stringify(remove, null, 2));
  for (const asset of remove) {
    const response = await fetch(base + asset.url);
    assert.equal(response.status, 200, `Cannot back up ${asset.label}`);
    await writeFile(new URL(`${asset.id}.${asset.format}`, backup), Buffer.from(await response.arrayBuffer()));
  }
  for (const asset of remove) {
    await request(`/api/model-assets/${asset.id}`, { method: "DELETE", headers }, 204);
    console.log(`Removed: ${asset.label}`);
  }
  const final = await request("/api/model-assets");
  assert.deepEqual(final.map((asset) => asset.id).sort(), published.map((asset) => asset.id).sort());
  console.log(`Library now contains exactly ${final.length} starter models.`);
}
await request("/api/auth/logout", { method: "POST", headers }, 204);
