import { useEffect, useRef } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { OBJLoader } from "three/examples/jsm/loaders/OBJLoader.js";
import { STLLoader } from "three/examples/jsm/loaders/STLLoader.js";
import { assetFileURL } from "./api";
import type { AssemblyPlacement, CharacterConfig, ModelAsset, Vector3Tuple } from "./types";

type Props = {
  config: CharacterConfig;
  modelAssets: ModelAsset[];
};

export default function CharacterPreview({ config, modelAssets }: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const configRef = useRef(config);
  const modelAssetsRef = useRef(modelAssets);
  const modelRef = useRef<THREE.Group | null>(null);
  const rebuildTokenRef = useRef(0);

  useEffect(() => {
    configRef.current = config;
    modelAssetsRef.current = modelAssets;
    if (modelRef.current) {
      const token = bumpRebuildToken(rebuildTokenRef);
      rebuildModel(modelRef.current, config, modelAssets, () => token === rebuildTokenRef.current);
    }
  }, [config, modelAssets]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }

    const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    renderer.setClearColor(0x111318, 1);
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.shadowMap.enabled = true;

    const scene = new THREE.Scene();
    scene.fog = new THREE.Fog(0x111318, 8, 18);

    const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 100);
    camera.position.set(3.4, 2.8, 5.4);

    const controls = new OrbitControls(camera, canvas);
    controls.enableDamping = true;
    controls.target.set(0, 1.2, 0);

    const ambient = new THREE.HemisphereLight(0xf8fafc, 0x2e3440, 2.1);
    scene.add(ambient);

    const key = new THREE.DirectionalLight(0xffffff, 2.8);
    key.position.set(4, 6, 4);
    key.castShadow = true;
    scene.add(key);

    const rim = new THREE.DirectionalLight(0x82d8ff, 1.8);
    rim.position.set(-5, 4, -3);
    scene.add(rim);

    const floor = new THREE.Mesh(
      new THREE.CircleGeometry(2.45, 96),
      new THREE.MeshStandardMaterial({ color: 0x23262d, roughness: 0.85, metalness: 0.08 }),
    );
    floor.rotation.x = -Math.PI / 2;
    floor.receiveShadow = true;
    scene.add(floor);

    const grid = new THREE.GridHelper(5.2, 16, 0x3d4654, 0x2a3039);
    grid.position.y = 0.01;
    scene.add(grid);

    const model = new THREE.Group();
    modelRef.current = model;
    scene.add(model);
    const initialToken = bumpRebuildToken(rebuildTokenRef);
    rebuildModel(model, configRef.current, modelAssetsRef.current, () => initialToken === rebuildTokenRef.current);

    const resize = () => {
      const parent = canvas.parentElement;
      const width = parent?.clientWidth ?? 640;
      const height = parent?.clientHeight ?? 640;
      renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
      renderer.setSize(width, height, false);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    };

    const observer = new ResizeObserver(resize);
    if (canvas.parentElement) {
      observer.observe(canvas.parentElement);
    }
    resize();

    let animationFrame = 0;
    const clock = new THREE.Clock();

    const animate = () => {
      const elapsed = clock.getElapsedTime();
      if (modelRef.current) {
        modelRef.current.rotation.y = Math.sin(elapsed * 0.3) * 0.08;
      }
      controls.update();
      renderer.render(scene, camera);
      animationFrame = requestAnimationFrame(animate);
    };
    animate();

    return () => {
      cancelAnimationFrame(animationFrame);
      observer.disconnect();
      controls.dispose();
      renderer.dispose();
      scene.traverse((object) => {
        if (object instanceof THREE.Mesh) {
          object.geometry.dispose();
          const materials = Array.isArray(object.material) ? object.material : [object.material];
          materials.forEach((material) => material.dispose());
        }
      });
      modelRef.current = null;
    };
  }, []);

  return <canvas ref={canvasRef} className="preview-canvas" aria-label="3D preview" />;
}

function rebuildModel(group: THREE.Group, config: CharacterConfig, modelAssets: ModelAsset[], isCurrent: () => boolean) {
  while (group.children.length) {
    const child = group.children.pop();
    if (child) {
      disposeObject(child);
    }
  }

  const scale = THREE.MathUtils.clamp(config.scale.mm / 32, 0.75, 1.5);
  group.scale.setScalar(scale);

  const skin = new THREE.MeshStandardMaterial({ color: config.colors.skin, roughness: 0.58 });
  const clothing = new THREE.MeshStandardMaterial({ color: config.colors.clothing_primary, roughness: 0.48, metalness: 0.25 });
  const hair = new THREE.MeshStandardMaterial({ color: config.colors.hair, roughness: 0.75 });
  const accent = new THREE.MeshStandardMaterial({ color: config.colors.clothing_secondary, roughness: 0.7 });
  const leather = new THREE.MeshStandardMaterial({ color: 0x4a3328, roughness: 0.8 });
  const metal = new THREE.MeshStandardMaterial({ color: config.colors.gear_metal, roughness: 0.36, metalness: 0.75 });
  const baseMaterial = new THREE.MeshStandardMaterial({ color: config.colors.stage, roughness: 0.82, metalness: 0.06 });

  const baseRadius = config.base.includes("40") ? 0.95 : 0.76;
  const base = new THREE.Mesh(new THREE.CylinderGeometry(baseRadius, baseRadius, 0.16, 64), baseMaterial);
  base.position.y = 0.08;
  base.receiveShadow = true;
  group.add(base);

  const bodyHeight = config.body.includes("compact") ? 1.28 : config.body.includes("heroic") ? 1.78 : 1.62;
  const torsoRadius = config.species.includes("orc") || config.body.includes("heroic") ? 0.42 : 0.34;
  const torso = new THREE.Mesh(new THREE.CapsuleGeometry(torsoRadius, bodyHeight - 0.65, 12, 24), skin);
  torso.position.y = 0.92;
  torso.castShadow = true;
  group.add(torso);

  const armorShape = config.clothing.includes("robe")
    ? new THREE.ConeGeometry(0.46, 0.9, 5)
    : config.clothing.includes("plate")
      ? new THREE.BoxGeometry(0.9, 0.82, 0.48)
      : new THREE.BoxGeometry(0.82, 0.74, 0.42);
  const chest = new THREE.Mesh(armorShape, clothing);
  chest.position.y = 1.12;
  chest.castShadow = true;
  group.add(chest);

  if (config.clothing.includes("cloak") || config.clothing.includes("robe")) {
    const cloak = new THREE.Mesh(new THREE.PlaneGeometry(0.95, 1.3), accent);
    cloak.position.set(0, 0.98, -0.28);
    cloak.rotation.x = -0.13;
    cloak.castShadow = true;
    group.add(cloak);
  }

  const headGeometry = config.head.includes("square")
    ? new THREE.BoxGeometry(0.46, 0.42, 0.42)
    : new THREE.SphereGeometry(0.25, 32, 24);
  const head = new THREE.Mesh(headGeometry, skin);
  head.position.y = bodyHeight + 0.72;
  head.castShadow = true;
  group.add(head);

  const hairGeometry = config.head.includes("noble")
    ? new THREE.ConeGeometry(0.32, 0.62, 24)
    : new THREE.SphereGeometry(0.27, 24, 12, 0, Math.PI * 2, 0, Math.PI * 0.52);
  const hairMesh = new THREE.Mesh(hairGeometry, hair);
  hairMesh.position.y = bodyHeight + (config.head.includes("noble") ? 0.53 : 0.9);
  hairMesh.castShadow = true;
  group.add(hairMesh);

  const armLeft = createLimb(leather, 0.12, 0.7);
  const armRight = createLimb(leather, 0.12, 0.7);
  armLeft.position.set(-0.53, 1.08, 0);
  armRight.position.set(0.53, 1.08, 0);
  armLeft.rotation.z = config.pose.includes("victory") ? -2.35 : config.pose.includes("guard") ? -1.15 : -0.45;
  armRight.rotation.z = config.pose.includes("victory") ? 2.35 : config.pose.includes("guard") ? 1.15 : 0.45;
  group.add(armLeft, armRight);

  const legLeft = createLimb(leather, 0.14, 0.78);
  const legRight = createLimb(leather, 0.14, 0.78);
  legLeft.position.set(-0.2, 0.48, 0);
  legRight.position.set(0.2, 0.48, 0);
  legLeft.rotation.z = 0.08;
  legRight.rotation.z = -0.08;
  group.add(legLeft, legRight);

  if (!config.gear.includes("none")) {
    const weapon = createWeapon(config.gear, metal, leather);
    weapon.position.set(0.78, 1.1, 0.05);
    weapon.rotation.z = config.pose.includes("guard") ? -0.7 : -0.18;
    weapon.rotation.x = 0.12;
    group.add(weapon);
  }

  if (config.stage.includes("dungeon") || config.stage.includes("ruins")) {
    const rubbleMaterial = new THREE.MeshStandardMaterial({ color: config.colors.stage, roughness: 0.9 });
    for (let index = 0; index < 5; index += 1) {
      const stone = new THREE.Mesh(new THREE.BoxGeometry(0.18, 0.08, 0.18), rubbleMaterial);
      stone.position.set(-0.52 + index * 0.26, 0.22, 0.48 + Math.sin(index) * 0.1);
      stone.rotation.y = index * 0.42;
      stone.castShadow = true;
      group.add(stone);
    }
  }

  addAssemblyPlacements(group, config, modelAssets, bodyHeight, isCurrent);
}

function createLimb(material: THREE.Material, radius: number, height: number) {
  const mesh = new THREE.Mesh(new THREE.CapsuleGeometry(radius, height, 8, 18), material);
  mesh.castShadow = true;
  return mesh;
}

function createWeapon(kind: string, metal: THREE.Material, grip: THREE.Material) {
  const weapon = new THREE.Group();
  const handle = new THREE.Mesh(new THREE.CylinderGeometry(0.035, 0.035, 0.88, 12), grip);
  handle.castShadow = true;
  weapon.add(handle);

  if (kind.includes("staff")) {
    handle.scale.set(1.25, 1.65, 1.25);
    const gem = new THREE.Mesh(new THREE.IcosahedronGeometry(0.13, 1), metal);
    gem.position.y = 0.78;
    gem.castShadow = true;
    weapon.add(gem);
  } else if (kind.includes("longbow")) {
    handle.scale.set(1, 1.55, 1);
    const bow = new THREE.Mesh(new THREE.TorusGeometry(0.34, 0.025, 8, 28, Math.PI), metal);
    bow.position.y = 0.42;
    bow.rotation.z = Math.PI / 2;
    bow.castShadow = true;
    weapon.add(bow);
  } else if (kind.includes("shield")) {
    const shield = new THREE.Mesh(new THREE.CylinderGeometry(0.28, 0.32, 0.08, 6), metal);
    shield.position.y = 0.28;
    shield.rotation.x = Math.PI / 2;
    shield.castShadow = true;
    weapon.add(shield);
  } else if (kind.includes("axe")) {
    const head = new THREE.Mesh(new THREE.BoxGeometry(0.35, 0.18, 0.08), metal);
    head.position.y = 0.38;
    head.castShadow = true;
    weapon.add(head);
  } else {
    const blade = new THREE.Mesh(new THREE.BoxGeometry(0.09, 0.72, 0.045), metal);
    blade.position.y = 0.54;
    blade.castShadow = true;
    weapon.add(blade);
  }

  return weapon;
}

function addAssemblyPlacements(
  group: THREE.Group,
  config: CharacterConfig,
  modelAssets: ModelAsset[],
  bodyHeight: number,
  isCurrent: () => boolean,
) {
  const assetsByID = new Map(modelAssets.map((asset) => [asset.id, asset]));
  for (const placement of config.assembly.placements) {
    const asset = assetsByID.get(placement.asset_id);
    const socket = new THREE.Group();
    applyPlacementTransform(socket, placement, bodyHeight, config.pose);
    group.add(socket);

    const placeholder = createUploadedPlaceholder(asset?.format ?? "missing");
    socket.add(placeholder);

    if (!asset || asset.format === "3mf") {
      continue;
    }

    void loadUploadedModel(asset)
      .then((object) => {
        if (!isCurrent()) {
          disposeObject(object);
          return;
        }
        socket.remove(placeholder);
        disposeObject(placeholder);
        socket.add(object);
      })
      .catch(() => {
        placeholder.traverse((child) => {
          if (child instanceof THREE.Mesh && child.material instanceof THREE.MeshStandardMaterial) {
            child.material.color.set(0xff6b6b);
          }
        });
      });
  }
}

function applyPlacementTransform(socket: THREE.Group, placement: AssemblyPlacement, bodyHeight: number, pose: string) {
  const base = attachmentBaseTransform(placement.attachment_point, bodyHeight, pose);
  const offset = tupleToVector3(placement.transform.position);
  socket.position.copy(base.position.add(offset));
  socket.rotation.set(
    base.rotation.x + THREE.MathUtils.degToRad(placement.transform.rotation[0]),
    base.rotation.y + THREE.MathUtils.degToRad(placement.transform.rotation[1]),
    base.rotation.z + THREE.MathUtils.degToRad(placement.transform.rotation[2]),
  );
  socket.scale.setScalar(placement.transform.scale);
}

function attachmentBaseTransform(point: string, bodyHeight: number, pose: string) {
  const guardTilt = pose.includes("guard") ? 0.45 : 0;
  switch (point) {
    case "body_core":
      return { position: new THREE.Vector3(0, 0.96, 0), rotation: new THREE.Euler(0, 0, 0) };
    case "head_mount":
      return { position: new THREE.Vector3(0, bodyHeight + 1.02, 0), rotation: new THREE.Euler(0, 0, 0) };
    case "torso_mount":
      return { position: new THREE.Vector3(0, 1.18, 0.24), rotation: new THREE.Euler(0, 0, 0) };
    case "back_mount":
      return { position: new THREE.Vector3(0, 1.16, -0.38), rotation: new THREE.Euler(0.1, Math.PI, 0) };
    case "left_hand":
      return { position: new THREE.Vector3(-0.78, 1.08, 0.04), rotation: new THREE.Euler(0.12, 0, 0.2 + guardTilt) };
    case "right_hand":
      return { position: new THREE.Vector3(0.78, 1.08, 0.04), rotation: new THREE.Euler(0.12, 0, -0.2 - guardTilt) };
    case "stage_anchor":
      return { position: new THREE.Vector3(0, 0.2, 0.62), rotation: new THREE.Euler(0, 0, 0) };
    case "base_mount":
      return { position: new THREE.Vector3(0, 0.22, 0), rotation: new THREE.Euler(0, 0, 0) };
    default:
      return { position: new THREE.Vector3(0, 1, 0), rotation: new THREE.Euler(0, 0, 0) };
  }
}

function createUploadedPlaceholder(format: string) {
  const material = new THREE.MeshStandardMaterial({
    color: format === "missing" ? 0xff6b6b : 0x6dc7ff,
    roughness: 0.42,
    metalness: 0.18,
    transparent: true,
    opacity: 0.78,
  });
  const group = new THREE.Group();
  const body = new THREE.Mesh(new THREE.BoxGeometry(0.42, 0.42, 0.42), material);
  body.castShadow = true;
  group.add(body);

  const wire = new THREE.Mesh(
    new THREE.BoxGeometry(0.45, 0.45, 0.45),
    new THREE.MeshBasicMaterial({ color: 0xf2b84b, wireframe: true }),
  );
  group.add(wire);
  return group;
}

async function loadUploadedModel(asset: ModelAsset): Promise<THREE.Object3D> {
  const url = assetFileURL(asset);
  if (asset.format === "glb" || asset.format === "gltf") {
    const gltf = await new GLTFLoader().loadAsync(url);
    return normalizeUploadedObject(gltf.scene);
  }
  if (asset.format === "obj") {
    const object = await new OBJLoader().loadAsync(url);
    return normalizeUploadedObject(object);
  }
  if (asset.format === "stl") {
    const geometry = await new STLLoader().loadAsync(url);
    const mesh = new THREE.Mesh(
      geometry,
      new THREE.MeshStandardMaterial({ color: 0xd8dee9, roughness: 0.36, metalness: 0.45 }),
    );
    mesh.castShadow = true;
    return normalizeUploadedObject(mesh);
  }
  return createUploadedPlaceholder(asset.format);
}

function normalizeUploadedObject(object: THREE.Object3D) {
  object.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.castShadow = true;
      child.receiveShadow = true;
      if (!child.material) {
        child.material = new THREE.MeshStandardMaterial({ color: 0xd8dee9, roughness: 0.5 });
      }
    }
  });

  const box = new THREE.Box3().setFromObject(object);
  const size = box.getSize(new THREE.Vector3());
  const center = box.getCenter(new THREE.Vector3());
  const maxAxis = Math.max(size.x, size.y, size.z) || 1;
  object.position.sub(center);
  object.scale.multiplyScalar(0.55 / maxAxis);

  const wrapper = new THREE.Group();
  wrapper.add(object);
  return wrapper;
}

function tupleToVector3(value: Vector3Tuple) {
  return new THREE.Vector3(value[0], value[1], value[2]);
}

function bumpRebuildToken(ref: { current: number }) {
  ref.current += 1;
  return ref.current;
}

function disposeObject(object: THREE.Object3D) {
  object.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.geometry.dispose();
      const materials = Array.isArray(child.material) ? child.material : [child.material];
      materials.forEach((material) => material.dispose());
    }
  });
}
