// Generates four original, self-contained GLB models. No textures or external resources.
// Run from the repository root: node scripts/create-starter-characters.mjs
import { mkdir, writeFile } from "node:fs/promises";
import * as THREE from "../frontend/node_modules/three/build/three.module.js";

const output = new URL("../assets/starter-characters/", import.meta.url);
await mkdir(output, { recursive: true });
const palette = {
  skin: [0.66, 0.39, 0.24, 1],
  suitMale: [0.11, 0.24, 0.28, 1],
  suitFemale: [0.33, 0.15, 0.23, 1],
  boots: [0.07, 0.09, 0.12, 1],
  trim: [0.57, 0.38, 0.13, 1],
  hair: [0.075, 0.037, 0.025, 1],
  eyes: [0.035, 0.045, 0.05, 1],
  lips: [0.39, 0.19, 0.14, 1],
};

function model() {
  const parts = [];
  function add(name, source, material, position = [0, 0, 0], scale = [1, 1, 1], rotation = [0, 0, 0]) {
    let geometry = source.index ? source.toNonIndexed() : source;
    geometry.applyMatrix4(new THREE.Matrix4().compose(new THREE.Vector3(...position),
      new THREE.Quaternion().setFromEuler(new THREE.Euler(...rotation)), new THREE.Vector3(...scale)));
    geometry.computeVertexNormals();
    parts.push({ name, geometry, material });
  }
  function oval(name, center, radius, material, segments = 12) {
    add(name, new THREE.SphereGeometry(1, segments, 8), material, center, radius);
  }
  function limb(name, start, end, rStart, rEnd, material) {
    const a = new THREE.Vector3(...start), b = new THREE.Vector3(...end);
    const direction = b.clone().sub(a);
    const geometry = new THREE.CylinderGeometry(rEnd, rStart, direction.length(), 12, 1);
    geometry.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), direction.normalize()));
    add(name, geometry, material, a.add(b).multiplyScalar(0.5).toArray());
  }
  function loft(name, rings, material, segments = 16) {
    const positions = [], indices = [];
    for (const [y, rx, rz, z = 0] of rings) {
      for (let i = 0; i < segments; i++) {
        const angle = i * Math.PI * 2 / segments;
        positions.push(Math.cos(angle) * rx, y, z + Math.sin(angle) * rz);
      }
    }
    for (let j = 0; j < rings.length - 1; j++) {
      for (let i = 0; i < segments; i++) {
        const a = j * segments + i, b = j * segments + (i + 1) % segments;
        indices.push(a, a + segments, b, b, a + segments, b + segments);
      }
    }
    // Seal both ends of each solid loft.
    for (const end of [0, rings.length - 1]) {
      const center = positions.length / 3;
      positions.push(0, rings[end][0], rings[end][3] ?? 0);
      for (let i = 0; i < segments; i++) {
        const a = end * segments + i, b = end * segments + (i + 1) % segments;
        indices.push(...(end === 0 ? [center, a, b] : [center, b, a]));
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    geometry.setIndex(indices);
    add(name, geometry, material);
  }
  return { parts, add, oval, limb, loft };
}

function makeBody(female) {
  const m = model(), suit = female ? "suitFemale" : "suitMale";
  // Bodies wear simple fitted adventurer's clothing, with boots and bare hands.
  m.loft("Torso and hips", female ? [
    [0.72, 0.13, 0.105], [0.83, 0.235, 0.15], [0.95, 0.225, 0.14],
    [1.08, 0.16, 0.115], [1.21, 0.205, 0.15, 0.015],
    [1.33, 0.25, 0.14], [1.39, 0.225, 0.115], [1.44, 0.085, 0.085],
  ] : [
    [0.72, 0.13, 0.11], [0.84, 0.205, 0.14], [0.98, 0.19, 0.125],
    [1.12, 0.215, 0.14], [1.27, 0.29, 0.17],
    [1.36, 0.315, 0.14], [1.4, 0.27, 0.115], [1.44, 0.095, 0.085],
  ], suit);
  const beltWidth = female ? 0.207 : 0.197;
  m.loft("Waist belt", [[0.99, beltWidth, 0.138], [1.025, beltWidth - 0.012, 0.131]], "boots");
  m.add("Belt buckle", new THREE.BoxGeometry(0.067, 0.044, 0.018), "trim", [0, 1.009, 0.14]);
  m.limb("Neck", [0, 1.4, 0], [0, 1.55, 0], female ? 0.079 : 0.086, 0.076, "skin");
  m.loft("Collar", [[1.425, 0.098, 0.088], [1.453, 0.089, 0.08]], "trim");
  for (const side of [-1, 1]) {
    const shoulder = [side * (female ? 0.25 : 0.307), 1.325, 0];
    const elbow = [side * (female ? 0.35 : 0.419), 1.025, 0.012];
    const wrist = [side * (female ? 0.416 : 0.483), 0.805, 0.035];
    m.oval("Shoulder", shoulder, female ? [0.102, 0.12, 0.105] : [0.124, 0.137, 0.125], suit);
    m.limb("Upper sleeve", shoulder, elbow, female ? 0.092 : 0.11, 0.066, suit);
    m.oval("Elbow", elbow, [0.069, 0.078, 0.075], suit);
    m.limb("Forearm sleeve", elbow, wrist, 0.079, 0.047, suit);
    m.oval("Cuff", wrist, [0.055, 0.035, 0.057], "trim");
    m.oval("Hand", [wrist[0] + side * 0.009, 0.732, 0.047], [0.052, 0.086, 0.043], "skin");
    m.oval("Thumb", [wrist[0] - side * 0.03, 0.735, 0.081], [0.026, 0.05, 0.022], "skin");
    const hip = [side * (female ? 0.128 : 0.112), 0.815, 0];
    const knee = [side * 0.134, 0.435, 0.022];
    const ankle = [side * 0.157, 0.13, 0.014];
    m.oval("Hip", hip, [female ? 0.119 : 0.109, 0.14, 0.12], suit);
    m.limb("Thigh", hip, knee, female ? 0.117 : 0.106, 0.079, suit);
    m.oval("Knee", knee, [0.082, 0.085, 0.088], suit);
    m.limb("Shin", knee, ankle, 0.086, 0.059, suit);
    m.limb("Boot shaft", [ankle[0], 0.28, 0.014], ankle, 0.084, 0.072, "boots");
    m.oval("Boot foot", [ankle[0], 0.058, 0.065], [0.081, 0.058, 0.147], "boots");
  }
  return m;
}

function makeHead(female) {
  const m = model();
  m.limb("Neck connector", [0, 0, 0], [0, 0.105, 0], 0.076, 0.082, "skin");
  m.loft("Face and skull", female ? [
    [0.07, 0.056, 0.06, 0.005], [0.115, 0.085, 0.09, 0.02],
    [0.19, 0.13, 0.112, 0.012], [0.285, 0.15, 0.122],
    [0.365, 0.143, 0.121, -0.005], [0.435, 0.121, 0.103, -0.012], [0.482, 0.058, 0.061, -0.015],
  ] : [
    [0.07, 0.073, 0.063, 0.008], [0.12, 0.124, 0.103, 0.017],
    [0.205, 0.143, 0.12, 0.006], [0.3, 0.159, 0.129],
    [0.38, 0.148, 0.122, -0.004], [0.446, 0.115, 0.1, -0.009], [0.484, 0.057, 0.056, -0.01],
  ], "skin", 16);
  m.oval("Nose", [0, 0.237, 0.128], [female ? 0.024 : 0.03, 0.047, 0.043], "skin", 8);
  m.oval("Mouth", [0, 0.168, 0.111], [0.046, 0.008, 0.011], "lips");
  for (const side of [-1, 1]) {
    m.oval("Ear", [side * 0.153, 0.243, -0.008], [0.034, 0.062, 0.028], "skin");
    m.oval("Eye", [side * 0.064, 0.286, 0.115], [0.023, 0.014, 0.011], "eyes");
    m.add("Eyebrow", new THREE.BoxGeometry(0.056, 0.011, 0.009), "hair", [side * 0.064, 0.321, 0.117], [1, 1, 1], [0, 0, side * -0.08]);
  }
  // Close-fitting hair cap; female bob frames the face without covering the connector.
  m.add("Hair cap", new THREE.SphereGeometry(1, 16, 8, 0, Math.PI * 2, 0, Math.PI * 0.46), "hair",
    [0, 0.373, -0.012], [female ? 0.166 : 0.163, 0.177, 0.142]);
  if (female) {
    for (const side of [-1, 1]) m.oval("Bob side", [side * 0.143, 0.275, -0.033], [0.055, 0.19, 0.117], "hair");
    m.oval("Bob back", [0, 0.28, -0.111], [0.155, 0.179, 0.052], "hair");
    m.oval("Swept fringe", [-0.055, 0.397, 0.09], [0.108, 0.057, 0.04], "hair");
  } else {
    m.oval("Short hair back", [0, 0.352, -0.109], [0.145, 0.094, 0.049], "hair");
    m.oval("Side swept fringe", [-0.045, 0.402, 0.086], [0.105, 0.037, 0.047], "hair");
  }
  return m;
}

function glb(name, parts) {
  const materialNames = Object.keys(palette);
  const doc = {
    asset: { version: "2.0", generator: "Taverna original starter-character generator" },
    scene: 0, scenes: [{ name, nodes: parts.map((_, i) => i) }], nodes: [], meshes: [],
    materials: materialNames.map((key) => ({ name: key, pbrMetallicRoughness: {
      baseColorFactor: palette[key], metallicFactor: key === "trim" ? 0.45 : 0, roughnessFactor: 0.82,
    } })), accessors: [], bufferViews: [], buffers: [{ byteLength: 0 }],
  };
  const chunks = [];
  let offset = 0;
  function attribute(values, bounds) {
    const bytes = Buffer.from(new Float32Array(values).buffer);
    const view = doc.bufferViews.length;
    doc.bufferViews.push({ buffer: 0, byteOffset: offset, byteLength: bytes.length, target: 34962 });
    chunks.push(bytes);
    offset += bytes.length;
    const accessor = { bufferView: view, componentType: 5126, count: values.length / 3, type: "VEC3" };
    if (bounds) Object.assign(accessor, { min: bounds.min.toArray(), max: bounds.max.toArray() });
    doc.accessors.push(accessor);
    return doc.accessors.length - 1;
  }
  for (const { name: partName, geometry, material } of parts) {
    geometry.computeBoundingBox();
    const position = attribute(geometry.getAttribute("position").array, geometry.boundingBox);
    const normal = attribute(geometry.getAttribute("normal").array);
    doc.nodes.push({ name: partName, mesh: doc.meshes.length });
    doc.meshes.push({ name: partName, primitives: [{ attributes: { POSITION: position, NORMAL: normal }, material: materialNames.indexOf(material), mode: 4 }] });
  }
  doc.buffers[0].byteLength = offset;
  const rawJson = Buffer.from(JSON.stringify(doc));
  const json = Buffer.concat([rawJson, Buffer.alloc((4 - rawJson.length % 4) % 4, 32)]);
  const binary = Buffer.concat(chunks);
  const header = Buffer.alloc(12);
  header.writeUInt32LE(0x46546c67); header.writeUInt32LE(2, 4); header.writeUInt32LE(28 + json.length + binary.length, 8);
  const jsonHeader = Buffer.alloc(8), binHeader = Buffer.alloc(8);
  jsonHeader.writeUInt32LE(json.length); jsonHeader.writeUInt32LE(0x4e4f534a, 4);
  binHeader.writeUInt32LE(binary.length); binHeader.writeUInt32LE(0x004e4942, 4);
  return Buffer.concat([header, jsonHeader, json, binHeader, binary]);
}

const entries = [];
for (const female of [false, true]) {
  for (const kind of ["body", "head"]) {
    const file = `${female ? "female" : "male"}-${kind}.glb`;
    const label = kind === "body" ? `${female ? "Женское" : "Мужское"} тело — базовое` : `${female ? "Женская" : "Мужская"} голова — сменная`;
    const object = kind === "body" ? makeBody(female) : makeHead(female);
    const bytes = glb(label, object.parts);
    await writeFile(new URL(file, output), bytes);
    const anchor = (id, label, position) => ({ id, label, position, rotation: [0, 0, 0] });
    entries.push({ file, label, slot: kind, attachment_point: kind === "body" ? "body_core" : "head_mount",
      assembly_points: kind === "body" ? [anchor("body_core", "Центр тела", [0, 0.8, 0]), anchor("head_mount", "Крепление головы", [0, 1.55, 0])]
        : [anchor("neck", "Основание шеи", [0, 0, 0])],
      triangles: object.parts.reduce((sum, part) => sum + part.geometry.getAttribute("position").count / 3, 0),
      bytes: bytes.length,
    });
  }
}
await writeFile(new URL("manifest.json", output), JSON.stringify(entries, null, 2) + "\n");
console.log(entries.map(({ file, triangles, bytes }) => ({ file, triangles, bytes })));
