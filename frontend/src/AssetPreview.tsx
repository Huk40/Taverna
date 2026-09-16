import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { assetFileURL } from "./api";
import { disposeModel, loadModel } from "./modelLoader";
import type { ModelAsset } from "./types";

type Props = { file?: File; asset?: ModelAsset; onReady?: (ready: boolean) => void };

export default function AssetPreview({ file, asset, onReady }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const readyCallback = useRef(onReady);
  readyCallback.current = onReady;
  const [message, setMessage] = useState("Загрузка предпросмотра…");
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    const container = host.current;
    if (!container || (!file && !asset)) return;
    let disposed = false;
    let frame = 0;
    let renderer: THREE.WebGLRenderer;
    setMessage("Загрузка предпросмотра…");
    setFailed(false);
    readyCallback.current?.(false);
    try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true }); }
    catch { setMessage("3D-предпросмотр недоступен: браузер не поддерживает WebGL."); setFailed(true); return; }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.setAttribute("aria-label", "Предпросмотр 3D-модели");
    container.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    scene.add(new THREE.HemisphereLight(0xffffff, 0x56677a, 2.5));
    const light = new THREE.DirectionalLight(0xffffff, 3);
    light.position.set(3, 5, 4);
    scene.add(light);
    const camera = new THREE.PerspectiveCamera(40, 1, 0.01, 100);
    camera.position.set(2.5, 1.8, 3);
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    const resize = () => {
      const width = Math.max(container.clientWidth, 1);
      const height = Math.max(container.clientHeight, 1);
      renderer.setSize(width, height);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    };
    const observer = new ResizeObserver(resize);
    observer.observe(container);
    resize();
    const url = file ? URL.createObjectURL(file) : assetFileURL(asset!);
    const format = file ? file.name.split(".").pop()!.toLowerCase() : asset!.format;
    void loadModel(url, format).then((object) => {
      if (disposed) { disposeModel(object); return; }
      const bounds = new THREE.Box3().setFromObject(object);
      const size = bounds.getSize(new THREE.Vector3());
      const dimension = Math.max(size.x, size.y, size.z);
      if (bounds.isEmpty() || !Number.isFinite(dimension) || dimension <= 0) {
        disposeModel(object);
        throw new Error("В файле нет видимой геометрии.");
      }
      // Fit the preview only; original coordinates are preserved in the library.
      const root = new THREE.Group();
      root.add(object);
      root.position.copy(bounds.getCenter(new THREE.Vector3())).multiplyScalar(-1);
      const normalized = new THREE.Group();
      normalized.add(root);
      normalized.scale.setScalar(2 / dimension);
      scene.add(normalized);
      setMessage("");
      readyCallback.current?.(true);
    }).catch((error: unknown) => {
      if (!disposed) {
        setFailed(true);
        setMessage(error instanceof Error ? error.message : "Не удалось прочитать модель.");
        readyCallback.current?.(false);
      }
    });
    const animate = () => { controls.update(); renderer.render(scene, camera); frame = requestAnimationFrame(animate); };
    animate();
    return () => {
      disposed = true;
      cancelAnimationFrame(frame);
      observer.disconnect();
      controls.dispose();
      disposeModel(scene);
      renderer.dispose();
      renderer.domElement.remove();
      if (file) URL.revokeObjectURL(url);
    };
  }, [file, asset]);
  return <div className="asset-preview-frame">
    <div ref={host} className="asset-preview-canvas" />
    {message && <p className="asset-preview-message" role={failed ? "alert" : "status"}>{message}</p>}
    {!message && <span className="asset-preview-hint">Вращайте мышью · Колесо — масштаб</span>}
  </div>;
}
