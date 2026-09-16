import { useMemo, useState } from "react";
import { Check, Eye, X } from "lucide-react";
import AssetPreview from "./AssetPreview";
import type { AssemblyPlacement, ModelAsset, SlotID } from "./types";

export const assetSlotLabels: Partial<Record<SlotID, string>> = { body: "Тело", head: "Голова", clothing: "Одежда и броня", gear: "Оружие и предметы", stage: "Окружение", base: "Подставка" };

type Props = { assets: ModelAsset[]; slot: SlotID; placements: AssemblyPlacement[]; onAdd: (asset: ModelAsset) => void; compact?: boolean; disabled?: boolean };

export default function AssetLibrary({ assets, slot, placements, onAdd, compact = false, disabled = false }: Props) {
  const [query, setQuery] = useState("");
  const [previewID, setPreviewID] = useState<string | null>(null);
  const models = useMemo(() => assets.filter((asset) => asset.slot === slot), [assets, slot]);
  const filtered = models.filter((asset) => asset.label.toLocaleLowerCase().includes(query.toLocaleLowerCase()));
  const title = assetSlotLabels[slot] ?? slot;
  return <section className="inspector-content model-picker" aria-label={`Выбор: ${title}`}>
    {!compact && <><div className="asset-title-row"><h2>{title}</h2><span className="mini-badge">{models.length}</span></div>
      <p className="empty-state">{slot === "body" ? "Выберите основу персонажа. Голову можно поменять отдельно."
        : slot === "head" ? "Обе головы подходят к мужскому и женскому телу." : "Выберите деталь для персонажа."}</p></>}
    {models.length > 6 && <label className="field"><span>Поиск</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Название модели…" /></label>}
    <div className="asset-list">{filtered.map((asset) => {
      const selected = placements.some((placement) => placement.asset_id === asset.id);
      return <article className={selected ? "model-asset-card selected" : "model-asset-card"} key={asset.id}>
        <button className="model-choice" type="button" disabled={disabled} aria-pressed={selected} onClick={() => onAdd(asset)}>
          <strong>{asset.label.replace(/ — (базовое|сменная)$/, "")}</strong>
          <span className="model-choice-state">{selected ? <><Check size={16} /> Выбрано</> : "Выбрать"}</span>
        </button>
        {!compact && previewID === asset.id && <AssetPreview asset={asset} />}
        {!compact && <button className="model-preview-toggle" type="button" onClick={() => setPreviewID(previewID === asset.id ? null : asset.id)}>
          {previewID === asset.id ? <X size={14} /> : <Eye size={14} />}{previewID === asset.id ? "Скрыть просмотр" : "Рассмотреть деталь"}
        </button>}
      </article>;
    })}</div>
    {filtered.length === 0 && <p className="empty-state">{models.length ? "Ничего не найдено." : "В этой категории пока нет моделей."}</p>}
  </section>;
}
