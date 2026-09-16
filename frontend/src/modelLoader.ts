import * as THREE from "three";
import { ThreeMFLoader } from "three/examples/jsm/loaders/3MFLoader.js";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { OBJLoader } from "three/examples/jsm/loaders/OBJLoader.js";
import { STLLoader } from "three/examples/jsm/loaders/STLLoader.js";

export async function loadModel(url: string, format: string): Promise<THREE.Object3D> {
  // A model is a single-file asset. Prevent loaders from requesting external
  // textures/buffers referenced by a file selected for preview.
  const manager = new THREE.LoadingManager();
  manager.setURLModifier((resource) => {
    if (resource === url || /^(data:|blob:)/i.test(resource)) return resource;
    throw new Error("Модель ссылается на внешний файл. Упакуйте текстуры и геометрию в один GLB.");
  });
  switch (format.toLowerCase()) {
    case "glb": case "gltf": return (await new GLTFLoader(manager).loadAsync(url)).scene;
    case "obj": return new OBJLoader(manager).loadAsync(url);
    case "3mf": return new ThreeMFLoader(manager).loadAsync(url);
    case "stl": return new THREE.Mesh(await new STLLoader(manager).loadAsync(url),
      new THREE.MeshStandardMaterial({ color: 0xd8dee9, roughness: 0.5, metalness: 0.15 }));
    default: throw new Error("Поддерживаются GLB, GLTF, OBJ, STL и 3MF.");
  }
}

export function disposeModel(object: THREE.Object3D) {
  const textures = new Set<THREE.Texture>();
  object.traverse((child) => {
    if (child instanceof THREE.Mesh || child instanceof THREE.LineSegments) {
      child.geometry.dispose();
      for (const material of Array.isArray(child.material) ? child.material : [child.material]) {
        for (const value of Object.values(material)) if (value instanceof THREE.Texture) textures.add(value);
        material.dispose();
      }
    }
  });
  textures.forEach((texture) => { texture.dispose(); });
}
