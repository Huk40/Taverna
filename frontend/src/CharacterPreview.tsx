import { useEffect, useRef } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import type { CharacterConfig } from "./types";

type Props = {
  config: CharacterConfig;
};

export default function CharacterPreview({ config }: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const configRef = useRef(config);
  const modelRef = useRef<THREE.Group | null>(null);

  useEffect(() => {
    configRef.current = config;
    if (modelRef.current) {
      rebuildModel(modelRef.current, config);
    }
  }, [config]);

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
    rebuildModel(model, configRef.current);

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

function rebuildModel(group: THREE.Group, config: CharacterConfig) {
  while (group.children.length) {
    const child = group.children.pop();
    if (child) {
      disposeObject(child);
    }
  }

  const scale = THREE.MathUtils.clamp(config.scale_mm / 32, 0.75, 1.5);
  group.scale.setScalar(scale);

  const skin = new THREE.MeshStandardMaterial({ color: 0xc99672, roughness: 0.58 });
  const armor = new THREE.MeshStandardMaterial({ color: config.colors.armor, roughness: 0.48, metalness: 0.25 });
  const hair = new THREE.MeshStandardMaterial({ color: config.colors.hair, roughness: 0.75 });
  const cape = new THREE.MeshStandardMaterial({ color: config.colors.cape, roughness: 0.7 });
  const leather = new THREE.MeshStandardMaterial({ color: 0x4a3328, roughness: 0.8 });
  const metal = new THREE.MeshStandardMaterial({ color: 0xd8dee9, roughness: 0.36, metalness: 0.75 });
  const baseMaterial = new THREE.MeshStandardMaterial({ color: 0x29313d, roughness: 0.82, metalness: 0.06 });

  const baseRadius = config.base.includes("40") ? 0.95 : 0.76;
  const base = new THREE.Mesh(new THREE.CylinderGeometry(baseRadius, baseRadius, 0.16, 64), baseMaterial);
  base.position.y = 0.08;
  base.receiveShadow = true;
  group.add(base);

  const bodyHeight = config.body.includes("dwarf") ? 1.25 : config.body.includes("female") ? 1.55 : 1.7;
  const torsoRadius = config.body.includes("orc") ? 0.4 : 0.34;
  const torso = new THREE.Mesh(new THREE.CapsuleGeometry(torsoRadius, bodyHeight - 0.65, 12, 24), skin);
  torso.position.y = 0.92;
  torso.castShadow = true;
  group.add(torso);

  const armorShape = config.armor.includes("robe")
    ? new THREE.ConeGeometry(0.46, 0.9, 5)
    : new THREE.BoxGeometry(0.82, 0.74, 0.42);
  const chest = new THREE.Mesh(armorShape, armor);
  chest.position.y = 1.12;
  chest.castShadow = true;
  group.add(chest);

  if (config.armor.includes("cape") || config.colors.cape !== "#8B0000") {
    const cloak = new THREE.Mesh(new THREE.PlaneGeometry(0.95, 1.3), cape);
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

  if (!config.hair.includes("bald")) {
    const hairGeometry = config.hair.includes("long")
      ? new THREE.ConeGeometry(0.32, 0.62, 24)
      : new THREE.SphereGeometry(0.27, 24, 12, 0, Math.PI * 2, 0, Math.PI * 0.52);
    const hairMesh = new THREE.Mesh(hairGeometry, hair);
    hairMesh.position.y = bodyHeight + (config.hair.includes("long") ? 0.53 : 0.9);
    hairMesh.castShadow = true;
    group.add(hairMesh);
  }

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

  if (!config.weapon.includes("none")) {
    const weapon = createWeapon(config.weapon, metal, leather);
    weapon.position.set(0.78, 1.1, 0.05);
    weapon.rotation.z = config.pose.includes("guard") ? -0.7 : -0.18;
    weapon.rotation.x = 0.12;
    group.add(weapon);
  }
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

function disposeObject(object: THREE.Object3D) {
  object.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.geometry.dispose();
      const materials = Array.isArray(child.material) ? child.material : [child.material];
      materials.forEach((material) => material.dispose());
    }
  });
}
