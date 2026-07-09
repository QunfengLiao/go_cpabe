import type { PolicyTreeNode } from "./tree/types";

export function PolicyJsonPreview({ tree }: { tree: PolicyTreeNode | null }) {
  return (
    <section className="json-preview-card">
      <pre>{tree ? JSON.stringify(tree, null, 2) : "{\n  \"type\": null\n}"}</pre>
    </section>
  );
}
