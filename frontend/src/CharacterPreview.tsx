import { useEffect, useRef } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { assetFileURL } from "./api";
import { loadModel, disposeModel } from "./modelLoader";
import type {
  AssemblyManifest,
  AssemblyPlacement,
  CharacterConfig,
  ModelAsset,
  ModelAssetAssemblyPoint,
  ResolvedAssemblyPlacement,
  Vector3Tuple,
} from "./types";

type Props = {
  assemblyManifest: AssemblyManifest | null;
  config: CharacterConfig;
  modelAssets: ModelAsset[];
  showAssemblyGuides?: boolean;
};

export default function CharacterPreview({ assemblyManifest, config, modelAssets, showAssemblyGuides = false }: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const assemblyManifestRef = useRef(assemblyManifest);
  const configRef = useRef(config);
  const modelAssetsRef = useRef(modelAssets);
  const showGuidesRef = useRef(showAssemblyGuides);
  const modelRef = useRef<THREE.Group | null>(null);
  const rebuildTokenRef = useRef(0);

  useEffect(() => {
    assemblyManifestRef.current = assemblyManifest;
    configRef.current = config;
    modelAssetsRef.current = modelAssets;
    showGuidesRef.current = showAssemblyGuides;
    if (modelRef.current) {
      const token = bumpRebuildToken(rebuildTokenRef);
      rebuildModel(modelRef.current, config, modelAssets, assemblyManifest, () => token === rebuildTokenRef.current, showAssemblyGuides);
    }
  }, [assemblyManifest, config, modelAssets, showAssemblyGuides]);

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
    rebuildModel(model, configRef.current, modelAssetsRef.current, assemblyManifestRef.current, () => initialToken === rebuildTokenRef.current, showGuidesRef.current);

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
      bumpRebuildToken(rebuildTokenRef);
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

function rebuildModel(
  group: THREE.Group,
  config: CharacterConfig,
  modelAssets: ModelAsset[],
  assemblyManifest: AssemblyManifest | null,
  isCurrent: () => boolean,
  showAssemblyGuides: boolean,
) {
  while (group.children.length) {
    const child = group.children.pop();
    if (child) {
      disposeObject(child);
    }
  }

  const scale = THREE.MathUtils.clamp(config.scale.mm / 32, 0.75, 1.5);
  group.scale.setScalar(scale);
  const baseMaterial = new THREE.MeshStandardMaterial({ color: config.colors.stage, roughness: 0.82, metalness: 0.06 });
  if (!config.assembly.placements.some((placement) => placement.slot === "base")) {
    const base = new THREE.Mesh(new THREE.CylinderGeometry(0.76, 0.76, 0.16, 64), baseMaterial);
    base.position.y = 0.08;
    base.receiveShadow = true;
    group.add(base);
  } else {
    baseMaterial.dispose();
  }
  addAssemblyPlacements(group, config, modelAssets, assemblyManifest, 1.62, scale, isCurrent, showAssemblyGuides);
}

function addAssemblyPlacements(
  group: THREE.Group,
  config: CharacterConfig,
  modelAssets: ModelAsset[],
  assemblyManifest: AssemblyManifest | null,
  bodyHeight: number,
  characterScale: number,
  isCurrent: () => boolean,
  showAssemblyGuides: boolean,
) {
  const assetsByID = new Map(modelAssets.map((asset) => [asset.id, asset]));
  const resolvedByID = new Map((assemblyManifest?.placements ?? []).map((placement) => [placement.id, placement]));
  // Match the server attachment while its debounced preview is still pending.
  let headBase: ReturnType<typeof attachmentBaseTransform> | undefined;
  let headParentScale = 1;
  for (const placement of config.assembly.placements) {
    const asset = assetsByID.get(placement.asset_id);
    const neck = asset?.assembly_points.find((point) => point.id === "head_mount");
    if (placement.slot !== "body" || !neck) continue;
    const body = new THREE.Group();
    applyPlacementTransform(body, placement, asset, bodyHeight, config.pose, characterScale, resolvedByID.get(placement.id));
    body.updateMatrix();
    headBase = {
      position: tupleToVector3(neck.position).applyMatrix4(body.matrix),
      rotation: new THREE.Euler(
        body.rotation.x + THREE.MathUtils.degToRad(neck.rotation[0]),
        body.rotation.y + THREE.MathUtils.degToRad(neck.rotation[1]),
        body.rotation.z + THREE.MathUtils.degToRad(neck.rotation[2]),
      ),
    };
    headParentScale = body.scale.x;
  }
  for (const placement of config.assembly.placements) {
    const asset = assetsByID.get(placement.asset_id);
    const resolvedPlacement = resolvedByID.get(placement.id);
    const socket = new THREE.Group();
    const isHead = placement.slot === "head" && placement.attachment_point === "head_mount";
    applyPlacementTransform(socket, placement, asset, bodyHeight, config.pose, characterScale, resolvedPlacement,
      isHead ? headBase : undefined, isHead && headBase ? headParentScale : 1);
    group.add(socket);
    if (showAssemblyGuides) addAssemblyGuides(group, socket, placement, asset, bodyHeight, config.pose, characterScale, resolvedPlacement);

    const placeholder = createUploadedPlaceholder(asset?.format ?? "missing");
    socket.add(placeholder);

    if (!asset) {
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

function addAssemblyGuides(
  group: THREE.Group,
  socket: THREE.Group,
  placement: AssemblyPlacement,
  asset: ModelAsset | undefined,
  bodyHeight: number,
  pose: string,
  characterScale: number,
  resolvedPlacement: ResolvedAssemblyPlacement | undefined,
) {
  const base = resolvedPlacement ? resolvedBaseTransform(resolvedPlacement) : attachmentBaseTransform(placement.attachment_point, bodyHeight, pose);
  const userTransform = resolvedPlacement?.user_transform ?? placement.transform;
  const targetMarker = createSocketTargetMarker();
  targetMarker.position.copy(base.position.clone().add(tupleToVector3(userTransform.position)));
  targetMarker.rotation.set(
    base.rotation.x + THREE.MathUtils.degToRad(userTransform.rotation[0]),
    base.rotation.y + THREE.MathUtils.degToRad(userTransform.rotation[1]),
    base.rotation.z + THREE.MathUtils.degToRad(userTransform.rotation[2]),
  );
  group.add(targetMarker);

  const assetPoint = resolvedPlacement?.asset_point ?? selectedAssetPoint(asset, placement.asset_point_id);
  const localPlacementScale = resolvedPlacement
    ? resolvedPlacement.world_transform.scale / Math.max(characterScale, 0.0001)
    : placement.transform.scale;
  const markerScale = 1 / Math.max(Math.abs(localPlacementScale), 0.05);
  const anchorPosition = tupleToVector3(assetPoint.position);

  const anchorMarker = createAssetAnchorMarker(markerScale);
  anchorMarker.position.copy(anchorPosition);
  anchorMarker.rotation.set(
    THREE.MathUtils.degToRad(assetPoint.rotation[0]),
    THREE.MathUtils.degToRad(assetPoint.rotation[1]),
    THREE.MathUtils.degToRad(assetPoint.rotation[2]),
  );
  socket.add(anchorMarker);

  const originMarker = createAssetOriginMarker(markerScale);
  socket.add(originMarker);

  const connector = createAnchorConnector(anchorPosition, markerScale);
  if (connector) {
    socket.add(connector);
  }
}

function createSocketTargetMarker() {
  const marker = new THREE.Group();
  const material = new THREE.MeshBasicMaterial({ color: 0xf2b84b, transparent: true, opacity: 0.9 });
  const ring = new THREE.Mesh(new THREE.TorusGeometry(0.09, 0.005, 8, 32), material);
  ring.rotation.x = Math.PI / 2;
  marker.add(ring);

  const dot = new THREE.Mesh(new THREE.SphereGeometry(0.025, 12, 8), material);
  marker.add(dot);
  return marker;
}

function createAssetAnchorMarker(scale: number) {
  const marker = new THREE.Group();
  marker.scale.setScalar(scale);

  const material = new THREE.MeshBasicMaterial({ color: 0x6dc7ff, transparent: true, opacity: 0.9 });
  const core = new THREE.Mesh(new THREE.IcosahedronGeometry(0.055, 1), material);
  marker.add(core);

  const ring = new THREE.Mesh(new THREE.TorusGeometry(0.09, 0.004, 8, 28), material);
  ring.rotation.y = Math.PI / 2;
  marker.add(ring);
  return marker;
}

function createAssetOriginMarker(scale: number) {
  const marker = new THREE.Mesh(
    new THREE.SphereGeometry(0.036 * scale, 12, 8),
    new THREE.MeshBasicMaterial({ color: 0x7ce2aa, transparent: true, opacity: 0.72 }),
  );
  return marker;
}

function createAnchorConnector(anchorPosition: THREE.Vector3, markerScale: number) {
  const distance = anchorPosition.length();
  if (distance < 0.001) {
    return null;
  }

  const material = new THREE.MeshBasicMaterial({ color: 0x7ce2aa, transparent: true, opacity: 0.6 });
  const connector = new THREE.Mesh(
    new THREE.CylinderGeometry(0.006 * markerScale, 0.006 * markerScale, distance, 8),
    material,
  );
  connector.position.copy(anchorPosition.clone().multiplyScalar(0.5));
  connector.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), anchorPosition.clone().normalize());
  return connector;
}

function applyPlacementTransform(
  socket: THREE.Group,
  placement: AssemblyPlacement,
  asset: ModelAsset | undefined,
  bodyHeight: number,
  pose: string,
  characterScale: number,
  resolvedPlacement: ResolvedAssemblyPlacement | undefined,
  baseOverride?: ReturnType<typeof attachmentBaseTransform>,
  parentScale = 1,
) {
  if (resolvedPlacement) {
    const world = resolvedPlacement.world_transform;
    const scale = Math.max(characterScale, 0.0001);
    socket.position.set(world.position[0] / scale, world.position[1] / scale, world.position[2] / scale);
    socket.rotation.set(
      THREE.MathUtils.degToRad(world.rotation[0]),
      THREE.MathUtils.degToRad(world.rotation[1]),
      THREE.MathUtils.degToRad(world.rotation[2]),
    );
    socket.scale.setScalar(world.scale / scale);
    return;
  }

  const base = baseOverride ?? attachmentBaseTransform(placement.attachment_point, bodyHeight, pose);
  const offset = tupleToVector3(placement.transform.position);
  const assetPoint = selectedAssetPoint(asset, placement.asset_point_id);
  socket.rotation.set(
    base.rotation.x + THREE.MathUtils.degToRad(placement.transform.rotation[0] - assetPoint.rotation[0]),
    base.rotation.y + THREE.MathUtils.degToRad(placement.transform.rotation[1] - assetPoint.rotation[1]),
    base.rotation.z + THREE.MathUtils.degToRad(placement.transform.rotation[2] - assetPoint.rotation[2]),
  );
  socket.scale.setScalar(placement.transform.scale * parentScale);
  const anchorOffset = tupleToVector3(assetPoint.position).multiplyScalar(placement.transform.scale * parentScale).applyEuler(socket.rotation);
  socket.position.copy(base.position.clone().add(offset).sub(anchorOffset));
}

function resolvedBaseTransform(placement: ResolvedAssemblyPlacement) {
  const socket = placement.socket_transform;
  return {
    position: tupleToVector3(socket.position),
    rotation: new THREE.Euler(
      THREE.MathUtils.degToRad(socket.rotation[0]),
      THREE.MathUtils.degToRad(socket.rotation[1]),
      THREE.MathUtils.degToRad(socket.rotation[2]),
    ),
  };
}

function selectedAssetPoint(asset: ModelAsset | undefined, requestedID: string): ModelAssetAssemblyPoint {
  const fallback: ModelAssetAssemblyPoint = {
    id: "origin",
    label: "Origin",
    position: [0, 0, 0],
    rotation: [0, 0, 0],
  };
  if (!asset?.assembly_points.length) {
    return fallback;
  }
  return asset.assembly_points.find((point) => point.id === requestedID) ?? asset.assembly_points[0] ?? fallback;
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
  const preserveCoordinates = asset.metadata?.status === "parsed";
  return prepareUploadedObject(await loadModel(assetFileURL(asset), asset.format), preserveCoordinates);
}

function prepareUploadedObject(object: THREE.Object3D, preserveCoordinates: boolean) {
  object.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.castShadow = true;
      child.receiveShadow = true;
      if (!child.material) {
        child.material = new THREE.MeshStandardMaterial({ color: 0xd8dee9, roughness: 0.5 });
      }
    }
  });

  if (!preserveCoordinates) {
    const box = new THREE.Box3().setFromObject(object);
    const size = box.getSize(new THREE.Vector3());
    const center = box.getCenter(new THREE.Vector3());
    const maxAxis = Math.max(size.x, size.y, size.z) || 1;
    object.position.sub(center);
    object.scale.multiplyScalar(0.55 / maxAxis);
  }

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
  disposeModel(object);
}
