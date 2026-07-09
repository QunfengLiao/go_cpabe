import { useEffect, useState } from "react";
import { createPlatformAttribute, createPlatformTemplate, deletePlatformAttribute, deletePlatformTemplate, listPlatformAttributes, listPlatformTemplates } from "../api/policy";
import { mockTree } from "../components/access-policy/tree/mockData";
import { attributeCode, attributeName, type PolicyAttribute, type PolicyTemplate } from "../components/access-policy/tree/types";

export function PlatformPolicyManagementPage() {
  const [attributes, setAttributes] = useState<PolicyAttribute[]>([]);
  const [templates, setTemplates] = useState<PolicyTemplate[]>([]);
  const [message, setMessage] = useState("");

  async function load() {
    const [nextAttrs, nextTemplates] = await Promise.all([listPlatformAttributes(), listPlatformTemplates()]);
    setAttributes(nextAttrs);
    setTemplates(nextTemplates);
  }

  useEffect(() => { void load().catch(() => setMessage("加载失败")); }, []);

  async function seedAttribute() {
    await createPlatformAttribute({ attrCode: "department", attrName: "部门", attrType: "enum", attrValues: ["研发部", "财务部"], status: "enabled" });
    await load();
  }

  async function seedTemplate() {
    await createPlatformTemplate({ name: "数据拥有者或租户管理员可访问", description: "常用模板", policyTreeJson: mockTree, status: "enabled" });
    await load();
  }

  return (
    <div className="access-policy-page">
      <div className="page-heading"><p>平台管理</p><h2>访问策略管理</h2></div>
      {message && <div className="access-tree-message">{message}</div>}
      <div className="platform-policy-grid">
        <section>
          <header><h3>属性字典</h3><button type="button" onClick={() => void seedAttribute().catch(() => setMessage("创建属性失败"))}>添加示例属性</button></header>
          {attributes.map((attr) => <article key={attr.id} className="policy-list-item"><strong>{attributeName(attr)}</strong><span>{attributeCode(attr)}</span><button type="button" onClick={() => void deletePlatformAttribute(attr.id).then(load)}>删除</button></article>)}
        </section>
        <section>
          <header><h3>策略模板</h3><button type="button" onClick={() => void seedTemplate().catch(() => setMessage("创建模板失败"))}>添加示例模板</button></header>
          {templates.map((template) => <article key={template.id} className="policy-list-item"><strong>{template.name}</strong><span>{template.description}</span><button type="button" onClick={() => void deletePlatformTemplate(template.id).then(load)}>删除</button></article>)}
        </section>
      </div>
    </div>
  );
}
