import { AssetCatalog } from "./AssetCatalog";
import { AssetDetailPage } from "./AssetDetailPage";

export { AssetCatalog, AssetDetailPage };

export function SkillsPage() {
  return <AssetCatalog kind="skills" />;
}

export function PluginsPage() {
  return <AssetCatalog kind="plugins" />;
}

export function ToolsPage() {
  return <AssetCatalog kind="tools" />;
}
